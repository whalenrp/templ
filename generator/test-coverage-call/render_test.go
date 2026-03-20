package testcoveragecall

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

func TestCallCoverage(t *testing.T) {
	snap := templruntime.CoverageSnapshot()
	if snap == nil {
		t.Skip("coverage not enabled (set TEMPLCOVERDIR)")
	}

	ctx := context.Background()
	var buf strings.Builder

	if err := CallsHelper().Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	snap = templruntime.CoverageSnapshot()
	points := snap["generator/test-coverage-call/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points for call site")
	}
	t.Logf("Coverage collected: %d points", len(points))
}
