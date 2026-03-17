package testcoveragebasic

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestCoverageIntegration(t *testing.T) {
	// Set up coverage directory
	coverageDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverageDir)

	// Enable coverage for testing (in case TEMPLCOVERDIR wasn't set before init)
	templruntime.EnableCoverageForTesting()

	// Render template with both branches
	var buf bytes.Buffer

	// Render with show=true
	if err := render(true).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render(true) failed: %v", err)
	}

	// Render with show=false
	buf.Reset()
	if err := render(false).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render(false) failed: %v", err)
	}

	// Explicitly flush coverage
	if err := templruntime.FlushCoverage(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	// Find profile file
	files, err := filepath.Glob(filepath.Join(coverageDir, "templ-*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("expected at least 1 profile file, found %d", len(files))
	}

	// Read profile
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("failed to read profile: %v", err)
	}

	var profile struct {
		Version string
		Mode    string
		Files   map[string][]struct {
			Line uint32
			Col  uint32
			Hits uint32
		}
	}

	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("failed to parse profile: %v", err)
	}

	// Verify coverage was collected
	// Note: The filename in the profile depends on how generator sets it
	// Check if any file has coverage points
	var totalPoints int
	for _, points := range profile.Files {
		totalPoints += len(points)
	}

	if totalPoints == 0 {
		t.Error("no coverage points recorded")
	}

	t.Logf("Coverage collected: %d points across %d files", totalPoints, len(profile.Files))

	// Verify at least some hits > 0 (templates were executed)
	var hasHits bool
	for _, points := range profile.Files {
		for _, pt := range points {
			if pt.Hits > 0 {
				hasHits = true
				break
			}
		}
	}

	if !hasHits {
		t.Error("no coverage points were hit (templates may not have executed)")
	}
}
