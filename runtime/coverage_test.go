package runtime

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// resetCoverageState resets global coverage state for isolated unit tests.
func resetCoverageState(t *testing.T) {
	t.Helper()
	old := coverageHits
	oldOut := coverageOut
	oldAppend := coverageAppend
	t.Cleanup(func() {
		coverageMu.Lock()
		defer coverageMu.Unlock()
		coverageHits = old
		coverageOut = oldOut
		coverageAppend = oldAppend
	})
	coverageMu.Lock()
	defer coverageMu.Unlock()
	coverageHits = nil
	coverageOut = ""
	coverageAppend = false
}

func TestCoverageTrack_NoOpWhenDisabled(t *testing.T) {
	resetCoverageState(t)
	// Should not panic.
	CoverageTrack("test.templ", 5, 10)
}

func TestCoverageTrack_RecordsWhenEnabled(t *testing.T) {
	resetCoverageState(t)
	coverageHits = make(map[string]map[coveragePos]uint32)

	CoverageTrack("test.templ", 5, 10)
	CoverageTrack("test.templ", 5, 10)
	CoverageTrack("test.templ", 7, 3)

	if got := CoverageHitAt("test.templ", 5, 10); got != 2 {
		t.Errorf("expected 2 hits at (5,10), got %d", got)
	}
	if got := CoverageHitAt("test.templ", 7, 3); got != 1 {
		t.Errorf("expected 1 hit at (7,3), got %d", got)
	}
}

func TestCoverageTrack_Concurrent(t *testing.T) {
	resetCoverageState(t)
	coverageHits = make(map[string]map[coveragePos]uint32)

	const goroutines, iterations = 100, 100
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				CoverageTrack("test.templ", 5, 10)
			}
		}()
	}
	wg.Wait()

	if got := CoverageHitAt("test.templ", 5, 10); got != goroutines*iterations {
		t.Errorf("expected %d hits, got %d (data race?)", goroutines*iterations, got)
	}
}

func TestCoverageHitAt_ZeroWhenDisabled(t *testing.T) {
	resetCoverageState(t)
	if got := CoverageHitAt("test.templ", 5, 10); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestInitCoverage_AutoDetectCoverprofile(t *testing.T) {
	resetCoverageState(t)

	// Simulate the flag the test binary receives when run with -coverprofile.
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	tmpFile := filepath.Join(t.TempDir(), "cov.out")
	os.Args = []string{"testbinary", "-test.coverprofile=" + tmpFile}

	initCoverage()

	if coverageHits == nil {
		t.Fatal("expected coverageHits to be initialised")
	}
	if coverageOut != tmpFile {
		t.Errorf("coverageOut = %q, want %q", coverageOut, tmpFile)
	}
	if !coverageAppend {
		t.Error("expected coverageAppend = true when auto-detected")
	}
}

func TestInitCoverage_TEMPLCOVERPROFILEEnvVar(t *testing.T) {
	resetCoverageState(t)
	tmpFile := filepath.Join(t.TempDir(), "templ.out")
	t.Setenv("TEMPLCOVERPROFILE", tmpFile)

	initCoverage()

	if coverageHits == nil {
		t.Fatal("expected coverageHits to be initialised")
	}
	if coverageOut != tmpFile {
		t.Errorf("coverageOut = %q, want %q", coverageOut, tmpFile)
	}
	if coverageAppend {
		t.Error("expected coverageAppend = false for TEMPLCOVERPROFILE")
	}
}

func TestInitCoverage_NoOpWithoutConfig(t *testing.T) {
	resetCoverageState(t)
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"testbinary"} // no -test.coverprofile
	t.Setenv("TEMPLCOVERPROFILE", "")

	initCoverage()

	if coverageHits != nil {
		t.Error("expected coverageHits to remain nil without config")
	}
}

func TestFlushCoverage_WritesStandardFormat(t *testing.T) {
	resetCoverageState(t)
	tmpFile := filepath.Join(t.TempDir(), "templ.out")
	coverageOut = tmpFile
	coverageAppend = false
	coverageHits = make(map[string]map[coveragePos]uint32)

	CoverageTrack("a.templ", 2, 3) // 0-indexed; expect 3.4 in profile (1-indexed)
	CoverageTrack("a.templ", 2, 3) // hit twice
	CoverageTrack("b.templ", 0, 0) // expect 1.1 in profile

	FlushCoverage()

	lines := readProfileLines(t, tmpFile)
	if len(lines) == 0 {
		t.Fatal("profile file is empty")
	}
	if lines[0] != "mode: count" {
		t.Errorf("first line = %q, want %q", lines[0], "mode: count")
	}

	// Check that our entries use 1-indexed line/col.
	assertHasEntry(t, lines, "a.templ:3.4,3.4 1 2")
	assertHasEntry(t, lines, "b.templ:1.1,1.1 1 1")
}

func TestFlushCoverage_AppendMode(t *testing.T) {
	resetCoverageState(t)
	tmpFile := filepath.Join(t.TempDir(), "cov.out")

	// Pre-write a "mode:" header as Go's coverage tool would.
	if err := os.WriteFile(tmpFile, []byte("mode: set\n"), 0644); err != nil {
		t.Fatal(err)
	}

	coverageOut = tmpFile
	coverageAppend = true
	coverageHits = make(map[string]map[coveragePos]uint32)
	CoverageTrack("t.templ", 4, 1)

	FlushCoverage()

	lines := readProfileLines(t, tmpFile)
	if lines[0] != "mode: set" {
		t.Errorf("first line = %q, want mode: set", lines[0])
	}
	// Our entry should appear after the existing header — no second mode line.
	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "mode:") {
			t.Errorf("unexpected extra mode line: %q", l)
		}
	}
	assertHasEntry(t, lines, "t.templ:5.2,5.2 1 1")
}

func TestRunWithCoverage_NoOpWithoutConfig(t *testing.T) {
	resetCoverageState(t)
	// No coverageHits, no coverageOut — should just pass through.
	code := RunWithCoverage(&mockRunner{code: 7})
	if code != 7 {
		t.Errorf("expected 7, got %d", code)
	}
}

func TestRunWithCoverage_FlushesOnReturn(t *testing.T) {
	resetCoverageState(t)
	tmpFile := filepath.Join(t.TempDir(), "templ.out")
	coverageOut = tmpFile
	coverageAppend = false
	coverageHits = make(map[string]map[coveragePos]uint32)

	CoverageTrack("x.templ", 1, 0)

	code := RunWithCoverage(&mockRunner{code: 0})
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	lines := readProfileLines(t, tmpFile)
	assertHasEntry(t, lines, "x.templ:2.1,2.1 1 1")
}

func TestRunWithCoverage_PropagatesExitCode(t *testing.T) {
	resetCoverageState(t)
	code := RunWithCoverage(&mockRunner{code: 42})
	if code != 42 {
		t.Errorf("expected 42, got %d", code)
	}
}

// helpers

type mockRunner struct{ code int }

func (m *mockRunner) Run() int { return m.code }

func readProfileLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open profile: %v", err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := sc.Text(); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func assertHasEntry(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, l := range lines {
		if l == want {
			return
		}
	}
	t.Errorf("profile missing entry %q\ngot lines: %v", want, lines)
}
