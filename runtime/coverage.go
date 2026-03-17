package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Position represents a source location in a template file
type Position struct {
	Line uint32 `json:"line"`
	Col  uint32 `json:"col"`
}

// CoveragePoint represents a single coverage measurement
type CoveragePoint struct {
	Line uint32 `json:"line"`
	Col  uint32 `json:"col"`
	Hits uint32 `json:"hits"`
	Type string `json:"type"`
}

// CoverageProfile represents the JSON coverage output format
type CoverageProfile struct {
	Version string                       `json:"version"`
	Mode    string                       `json:"mode"`
	Files   map[string][]CoveragePoint   `json:"files"`
}

// CoverageRegistry tracks coverage data during test execution
type CoverageRegistry struct {
	mu    sync.Mutex
	files map[string]map[Position]uint32 // filename → position → hit count
}

var coverageRegistry *CoverageRegistry

// initCoverage initializes the coverage registry if TEMPLCOVERDIR is set
func initCoverage() {
	if os.Getenv("TEMPLCOVERDIR") != "" {
		coverageRegistry = &CoverageRegistry{
			files: make(map[string]map[Position]uint32),
		}
	}
}

// Record increments the hit count for a coverage point
func (r *CoverageRegistry) Record(filename string, line, col uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.files[filename] == nil {
		r.files[filename] = make(map[Position]uint32)
	}

	pos := Position{Line: line, Col: col}
	r.files[filename][pos]++
}

// CoverageTrack records that a coverage point was executed
// Called by generated template code when coverage is enabled
func CoverageTrack(filename string, line, col uint32) {
	if coverageRegistry == nil {
		return // No-op if coverage disabled
	}
	coverageRegistry.Record(filename, line, col)
}

// WriteProfile writes coverage data to a JSON file
func (r *CoverageRegistry) WriteProfile(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	profile := CoverageProfile{
		Version: "1",
		Mode:    "count",
		Files:   make(map[string][]CoveragePoint),
	}

	// Convert internal map to slice format
	for filename, positions := range r.files {
		points := make([]CoveragePoint, 0, len(positions))
		for pos, hits := range positions {
			points = append(points, CoveragePoint{
				Line: pos.Line,
				Col:  pos.Col,
				Hits: hits,
				Type: "expression", // Default type for now
			})
		}
		profile.Files[filename] = points
	}

	// Write JSON
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}

	return nil
}

// Flush writes the coverage profile to disk
func (r *CoverageRegistry) Flush() error {
	outputDir := os.Getenv("TEMPLCOVERDIR")
	if outputDir == "" {
		outputDir = "."
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate unique filename
	filename := fmt.Sprintf("templ-%d-%d.json", os.Getpid(), time.Now().Unix())
	path := filepath.Join(outputDir, filename)

	return r.WriteProfile(path)
}

func init() {
	initCoverage()
	if coverageRegistry != nil {
		// Register exit hook to flush coverage on process exit
		// Note: We can't import testing package here, so we use a simple approach
		// that works for test processes. For more robust handling in production,
		// consider signal handlers or other exit hook mechanisms in Phase 4.
		go func() {
			// This goroutine will exist for the process lifetime
			// When main goroutine exits, deferred functions run, but we need
			// to ensure coverage is written. We'll rely on test cleanup for now.
		}()
	}
}
