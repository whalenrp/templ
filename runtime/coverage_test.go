package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCoverageRegistry_InitializesWhenEnvSet(t *testing.T) {
	t.Setenv("TEMPLCOVERDIR", t.TempDir())

	// Reset global state
	coverageRegistry = nil
	initCoverage()

	if coverageRegistry == nil {
		t.Error("expected registry to initialize when TEMPLCOVERDIR set")
	}
}

func TestCoverageRegistry_NilWhenEnvUnset(t *testing.T) {
	os.Unsetenv("TEMPLCOVERDIR")

	// Reset global state
	coverageRegistry = nil
	initCoverage()

	if coverageRegistry != nil {
		t.Error("expected registry to be nil when TEMPLCOVERDIR unset")
	}
}

func TestCoverageRegistry_Record(t *testing.T) {
	reg := &CoverageRegistry{
		files: make(map[string]map[Position]uint32),
	}

	// Record same position twice
	reg.Record("test.templ", 5, 10)
	reg.Record("test.templ", 5, 10)

	// Record different position
	reg.Record("test.templ", 7, 3)

	// Verify hit counts
	pos1 := Position{Line: 5, Col: 10}
	if hits := reg.files["test.templ"][pos1]; hits != 2 {
		t.Errorf("expected 2 hits for position (5,10), got %d", hits)
	}

	pos2 := Position{Line: 7, Col: 3}
	if hits := reg.files["test.templ"][pos2]; hits != 1 {
		t.Errorf("expected 1 hit for position (7,3), got %d", hits)
	}
}

func TestCoverageRegistry_RecordConcurrent(t *testing.T) {
	reg := &CoverageRegistry{
		files: make(map[string]map[Position]uint32),
	}

	// Concurrent writes to same position
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				reg.Record("test.templ", 5, 10)
			}
		}()
	}

	wg.Wait()

	pos := Position{Line: 5, Col: 10}
	expected := uint32(goroutines * iterations)
	if hits := reg.files["test.templ"][pos]; hits != expected {
		t.Errorf("expected %d hits, got %d (data race?)", expected, hits)
	}
}

func TestCoverageTrack_NoOpWhenDisabled(t *testing.T) {
	// Save and restore global state
	oldRegistry := coverageRegistry
	t.Cleanup(func() { coverageRegistry = oldRegistry })

	coverageRegistry = nil

	// Should not panic
	CoverageTrack("test.templ", 5, 10)
}

func TestCoverageTrack_RecordsWhenEnabled(t *testing.T) {
	// Save and restore global state
	oldRegistry := coverageRegistry
	t.Cleanup(func() { coverageRegistry = oldRegistry })

	coverageRegistry = &CoverageRegistry{
		files: make(map[string]map[Position]uint32),
	}

	CoverageTrack("test.templ", 5, 10)

	pos := Position{Line: 5, Col: 10}
	if hits := coverageRegistry.files["test.templ"][pos]; hits != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}
}

func TestCoverageRegistry_WriteProfile(t *testing.T) {
	reg := &CoverageRegistry{
		files: make(map[string]map[Position]uint32),
	}

	reg.Record("test.templ", 5, 10)
	reg.Record("test.templ", 7, 3)
	reg.Record("other.templ", 2, 1)

	tmpFile := filepath.Join(t.TempDir(), "profile.json")

	if err := reg.WriteProfile(tmpFile); err != nil {
		t.Fatalf("WriteProfile failed: %v", err)
	}

	// Read and parse JSON
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read profile: %v", err)
	}

	var profile CoverageProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Verify structure
	if profile.Version != "1" {
		t.Errorf("expected version 1, got %s", profile.Version)
	}

	if profile.Mode != "count" {
		t.Errorf("expected mode count, got %s", profile.Mode)
	}

	// Verify test.templ has 2 coverage points
	if len(profile.Files["test.templ"]) != 2 {
		t.Errorf("expected 2 points for test.templ, got %d", len(profile.Files["test.templ"]))
	}

	// Verify other.templ has 1 coverage point
	if len(profile.Files["other.templ"]) != 1 {
		t.Errorf("expected 1 point for other.templ, got %d", len(profile.Files["other.templ"]))
	}
}

func TestCoverageRegistry_Flush(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", tmpDir)

	reg := &CoverageRegistry{
		files: make(map[string]map[Position]uint32),
	}
	reg.Record("test.templ", 5, 10)

	if err := reg.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify file was created
	files, err := filepath.Glob(filepath.Join(tmpDir, "templ-*.json"))
	if err != nil {
		t.Fatalf("failed to glob files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 profile file, found %d", len(files))
	}

	// Verify content
	data, _ := os.ReadFile(files[0])
	var profile CoverageProfile
	json.Unmarshal(data, &profile)

	if len(profile.Files["test.templ"]) != 1 {
		t.Errorf("expected 1 coverage point in profile")
	}
}
