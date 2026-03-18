package testcoverageintegration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a-h/templ"
	templruntime "github.com/a-h/templ/runtime"
)

func TestIntegrationCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	// Exercise all code paths
	tests := []struct {
		name string
		comp templ.Component
	}{
		{"if-then", Comprehensive(true, []string{"a"})},
		{"if-else", Comprehensive(false, []string{})},
		{"switch-case0", Comprehensive(true, []string{})},
		{"switch-case1", Comprehensive(true, []string{"a"})},
		{"switch-default", Comprehensive(true, []string{"a", "b"})},
		{"children", Main()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			if err := tc.comp.Render(ctx, &buf); err != nil {
				t.Fatal(err)
			}
		})
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

	points := profile.Files["generator/test-coverage-integration/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	t.Logf("Total coverage points: %d", len(points))

	if len(points) < 15 {
		t.Errorf("expected at least 15 coverage points, got %d", len(points))
	}

	// Verify hit counts make sense
	for _, point := range points {
		if point.Hits == 0 {
			t.Errorf("coverage point at %d:%d was never hit", point.Line, point.Col)
		}
	}
}
