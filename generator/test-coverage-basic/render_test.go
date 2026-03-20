package testcoveragebasic

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

func TestCoverageIntegration(t *testing.T) {
	snap := templruntime.CoverageSnapshot()
	if snap == nil {
		t.Skip("coverage not enabled (set TEMPLCOVERDIR)")
	}

	ctx := context.Background()
	var buf strings.Builder

	if err := render(true).Render(ctx, &buf); err != nil {
		t.Fatalf("render(true) failed: %v", err)
	}

	buf.Reset()
	if err := render(false).Render(ctx, &buf); err != nil {
		t.Fatalf("render(false) failed: %v", err)
	}

	snap = templruntime.CoverageSnapshot()

	var totalPoints int
	var hasHits bool
	for _, points := range snap {
		totalPoints += len(points)
		for _, pt := range points {
			if pt.Hits > 0 {
				hasHits = true
			}
		}
	}

	if totalPoints == 0 {
		t.Error("no coverage points recorded")
	}
	if !hasHits {
		t.Error("no coverage points were hit")
	}

	t.Logf("Coverage collected: %d points across %d files", totalPoints, len(snap))
}
