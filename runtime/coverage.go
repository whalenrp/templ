package runtime

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// coverageState holds all runtime coverage state.
// Initialised once by init() or by tests via resetCoverageState.
var (
	coverageMu     sync.Mutex
	coverageHits   map[string]map[coveragePos]uint32
	coverageOut    string // file to write profile to
	coverageAppend bool   // true: Go already wrote the "mode:" header, just append
)

type coveragePos struct{ line, col uint32 }

func init() {
	initCoverage()
}

// initCoverage wires up coverage output. Separated from init so tests can reset state.
func initCoverage() {
	// Auto-detect: if the test binary was given -test.coverprofile, append our
	// blocks to that same file so users need no extra env vars.
	for _, arg := range os.Args {
		if v, ok := strings.CutPrefix(arg, "-test.coverprofile="); ok {
			coverageOut = v
			coverageAppend = true
			coverageHits = make(map[string]map[coveragePos]uint32)
			return
		}
	}
	// Explicit override: write a standalone profile (includes "mode:" header).
	if p := os.Getenv("TEMPLCOVERPROFILE"); p != "" {
		coverageOut = p
		coverageAppend = false
		coverageHits = make(map[string]map[coveragePos]uint32)
	}
}

// CoverageTrack records that a coverage point was executed.
// Called by generated template code; no-op when coverage is not configured.
func CoverageTrack(filename string, line, col uint32) {
	if coverageHits == nil {
		return
	}
	coverageMu.Lock()
	defer coverageMu.Unlock()
	if coverageHits[filename] == nil {
		coverageHits[filename] = make(map[coveragePos]uint32)
	}
	coverageHits[filename][coveragePos{line, col}]++
}

// CoverageHitAt returns the hit count for a specific coverage point.
// Returns 0 if coverage is disabled or the point was never hit.
// Intended for use in tests that verify coverage behaviour in-process.
func CoverageHitAt(filename string, line, col uint32) uint32 {
	if coverageHits == nil {
		return 0
	}
	coverageMu.Lock()
	defer coverageMu.Unlock()
	if m, ok := coverageHits[filename]; ok {
		return m[coveragePos{line, col}]
	}
	return 0
}

// TestRunner is implemented by *testing.M.
type TestRunner interface {
	Run() int
}

// RunWithCoverage wraps m.Run() to ensure coverage is written before the
// process exits. Safe to leave permanently — it is a no-op when coverage is
// not configured.
func RunWithCoverage(m TestRunner) int {
	code := m.Run()
	FlushCoverage()
	return code
}

// EnableCoverageForTest initialises in-process coverage tracking for tests that
// want to verify hit counts via CoverageHitAt. Call this in TestMain before
// RunWithCoverage. If outPath is non-empty it overrides the output file;
// if empty, any auto-detected path (from -test.coverprofile) is preserved.
func EnableCoverageForTest(outPath string) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	coverageHits = make(map[string]map[coveragePos]uint32)
	if outPath != "" {
		coverageOut = outPath
		coverageAppend = false
	}
	// If outPath is empty, keep whatever auto-detect set up in init().
}

// FlushCoverage writes the current coverage state to the configured output
// file. Calling it multiple times appends additional entries; the merge tools
// handle deduplication. Exported so tests can flush mid-run if needed.
func FlushCoverage() {
	if coverageHits == nil || coverageOut == "" {
		return
	}
	coverageMu.Lock()
	defer coverageMu.Unlock()

	var f *os.File
	var err error
	if coverageAppend {
		// Go has already written "mode: set" — just append our blocks.
		f, err = os.OpenFile(coverageOut, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		f, err = os.Create(coverageOut)
		if err == nil {
			fmt.Fprintln(f, "mode: count")
		}
	}
	if err != nil {
		return
	}
	defer f.Close()

	for filename, positions := range coverageHits {
		for pos, count := range positions {
			// Standard Go coverage format. The parser uses 0-indexed line/col;
			// the standard format is 1-indexed, so we add 1 to each.
			fmt.Fprintf(f, "%s:%d.%d,%d.%d 1 %d\n",
				filename,
				pos.line+1, pos.col+1,
				pos.line+1, pos.col+1,
				count)
		}
	}
}
