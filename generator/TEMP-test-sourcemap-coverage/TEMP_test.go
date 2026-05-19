// TEMP: exercise template for source-map coverage prototype (no TEMPLCOVERDIR instrumentation).
package tempsourcemapcoverage

import (
	"context"
	"strings"
	"testing"
)

func TestTEMP_SourceMapCoverage(t *testing.T) {
	ctx := context.Background()
	var buf strings.Builder

	cases := []struct {
		name  string
		show  bool
		items []string
	}{
		{"if-then", true, []string{"a", "b"}},
		{"if-else", false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			if err := Comprehensive(tc.show, tc.items).Render(ctx, &buf); err != nil {
				t.Fatal(err)
			}
		})
	}
}
