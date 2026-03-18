package testcoveragecall

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestCallCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	if err := CallsHelper().Render(ctx, &buf); err != nil {
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

	points := profile.Files["generator/test-coverage-call/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points for call site")
	}
	t.Logf("Coverage collected: %d points", len(points))
}
