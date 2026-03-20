package testcoverageintegration

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/a-h/templ"
	templruntime "github.com/a-h/templ/runtime"
)

func TestMain(m *testing.M) {
	os.Exit(templruntime.RunWithCoverage(m))
}

func TestIntegrationCoverage(t *testing.T) {
	snap := templruntime.CoverageSnapshot()
	if snap == nil {
		t.Skip("coverage not enabled (set TEMPLCOVERDIR)")
	}

	ctx := context.Background()
	var buf strings.Builder

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

	snap = templruntime.CoverageSnapshot()
	points := snap["generator/test-coverage-integration/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	t.Logf("Total coverage points: %d", len(points))

	if len(points) < 15 {
		t.Errorf("expected at least 15 coverage points, got %d", len(points))
	}

	for _, point := range points {
		if point.Hits == 0 {
			t.Errorf("coverage point at %d:%d was never hit", point.Line, point.Col)
		}
	}
}
