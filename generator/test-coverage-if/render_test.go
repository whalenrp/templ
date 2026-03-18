package testcoverageif

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestCoverageIf(t *testing.T) {
	coverageDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverageDir)
	templruntime.EnableCoverageForTesting()

	var buf bytes.Buffer

	if err := WithIf(true).Render(context.Background(), &buf); err != nil {
		t.Fatalf("WithIf(true) failed: %v", err)
	}
	buf.Reset()
	if err := WithIf(false).Render(context.Background(), &buf); err != nil {
		t.Fatalf("WithIf(false) failed: %v", err)
	}
	buf.Reset()
	if err := WithIfElse(true).Render(context.Background(), &buf); err != nil {
		t.Fatalf("WithIfElse(true) failed: %v", err)
	}
	buf.Reset()
	if err := WithIfElse(false).Render(context.Background(), &buf); err != nil {
		t.Fatalf("WithIfElse(false) failed: %v", err)
	}

	if err := templruntime.FlushCoverage(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(coverageDir, "templ-*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("expected at least 1 profile file, found %d", len(files))
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("failed to read profile: %v", err)
	}

	var profile templruntime.CoverageProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("failed to parse profile: %v", err)
	}

	var totalPoints int
	for _, points := range profile.Files {
		totalPoints += len(points)
	}

	// WithIf has 2 coverage points (condition + then-branch)
	// WithIfElse has 3 coverage points (condition + then-branch + else-branch)
	// Total: 5 points
	if totalPoints < 5 {
		t.Errorf("expected at least 5 coverage points, got %d", totalPoints)
	}

	t.Logf("Coverage collected: %d points across %d files", totalPoints, len(profile.Files))
}
