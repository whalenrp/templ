// TEMP: prototype - not for commit; tests //line vs source-map approach
package testcoverageintegration

import (
	"context"
	"strings"
	"testing"
)

// TEMP_TestCoverageBaseline exercises the template under standard go test -cover
// (no TEMPLCOVERDIR, just a plain coverage profile against the generated .go file).
func TestTEMP_CoverageBaseline(t *testing.T) {
	ctx := context.Background()
	var buf strings.Builder

	cases := []struct {
		name string
		show bool
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
