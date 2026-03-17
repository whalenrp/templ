package runtime

import (
	"os"
	"sync"
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
