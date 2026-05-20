package testcoverageintegration

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/a-h/templ"
	templruntime "github.com/a-h/templ/runtime"
)

const templFile = "generator/test-coverage-integration/template.templ"

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "templ-coverage-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	templruntime.EnableCoverageForTest("") // in-memory only; no file output needed
	os.Exit(templruntime.RunWithCoverage(m))
}

func TestIntegrationCoverage(t *testing.T) {
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

	// Verify that coverage is active and recording hits.
	if templruntime.CoverageHitAt(templFile, 7, 1) == 0 {
		t.Error("expected Comprehensive (line 7) to have been hit")
	}
	if templruntime.CoverageHitAt(templFile, 3, 1) == 0 {
		t.Error("expected Helper (line 3) to have been hit")
	}

	// if show { true branch — exercised by if-then, switch-case1, switch-default
	if templruntime.CoverageHitAt(templFile, 11, 2) == 0 {
		t.Error("expected if-show true branch to have been hit")
	}

	// else branch — exercised by if-else
	if templruntime.CoverageHitAt(templFile, 14, 3) == 0 {
		t.Error("expected if-show else branch to have been hit")
	}

	// for loop — exercised by switch-case1, switch-default
	if templruntime.CoverageHitAt(templFile, 24, 3) == 0 {
		t.Error("expected for-range loop to have been hit")
	}
}
