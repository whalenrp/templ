package testcoveragefor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestForCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	if err := WithFor([]string{"a", "b"}).Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	if err := WithEmptyFor().Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	if err := templruntime.FlushCoverage(); err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob(filepath.Join(coverDir, "templ-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no coverage profile generated")
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}

	var profile templruntime.CoverageProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}

	points := profile.Files["generator/test-coverage-for/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	if len(points) < 3 {
		t.Errorf("expected at least 3 coverage points, got %d", len(points))
	}
	t.Logf("Coverage collected: %d points", len(points))
}
