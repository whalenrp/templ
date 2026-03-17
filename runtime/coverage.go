package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
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

var (
	coverageRegistry *CoverageRegistry
	signalOnce       sync.Once
)

// initCoverage initializes the coverage registry if TEMPLCOVERDIR is set
func initCoverage() {
	if os.Getenv("TEMPLCOVERDIR") != "" {
		coverageRegistry = &CoverageRegistry{
			files: make(map[string]map[Position]uint32),
		}
	}
}

// EnableCoverageForTesting initializes coverage for tests
// Call this in tests before rendering templates if TEMPLCOVERDIR wasn't set before init
func EnableCoverageForTesting() {
	if coverageRegistry == nil {
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

// FlushCoverage explicitly flushes coverage data to disk
// Tests should call this in cleanup to ensure profiles are written
func FlushCoverage() error {
	if coverageRegistry == nil {
		return nil
	}
	return coverageRegistry.Flush()
}

func init() {
	initCoverage()
	if coverageRegistry != nil {
		// Best-effort auto-flush on interrupt signals (once per process)
		signalOnce.Do(func() {
			go func() {
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
				<-sigChan
				FlushCoverage()
				os.Exit(1)
			}()
		})
	}
}
