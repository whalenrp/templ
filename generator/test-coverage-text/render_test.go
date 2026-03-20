package testcoveragetext

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

func TestTextCoverage(t *testing.T) {
	snap := templruntime.CoverageSnapshot()
	if snap == nil {
		t.Skip("coverage not enabled (set TEMPLCOVERDIR)")
	}

	ctx := context.Background()
	var buf strings.Builder

	if err := WithText().Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	snap = templruntime.CoverageSnapshot()
	points := snap["generator/test-coverage-text/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}
	t.Logf("Coverage collected: %d points", len(points))
}
