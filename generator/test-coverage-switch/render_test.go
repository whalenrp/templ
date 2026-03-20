package testcoverageswitch

import (
	"context"
	"os"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestMain(m *testing.M) {
	os.Exit(templruntime.RunWithCoverage(m))
}

func TestSwitchCoverage(t *testing.T) {
	snap := templruntime.CoverageSnapshot()
	if snap == nil {
		t.Skip("coverage not enabled (set TEMPLCOVERDIR)")
	}

	ctx := context.Background()
	var buf strings.Builder

	for _, val := range []string{"a", "b", "other"} {
		buf.Reset()
		if err := WithSwitch(val).Render(ctx, &buf); err != nil {
			t.Fatal(err)
		}
	}

	snap = templruntime.CoverageSnapshot()
	points := snap["generator/test-coverage-switch/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	if len(points) < 4 {
		t.Errorf("expected at least 4 coverage points, got %d", len(points))
	}
	t.Logf("Coverage collected: %d points", len(points))
}
