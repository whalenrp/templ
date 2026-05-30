package generator

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ/parser/v2"
)

// templSource is a fixture covering every executable construct that should
// appear in the source map. Line numbers are 0-indexed.
//
//	line 0:  package sourcemaptest
//	line 1:  (blank)
//	line 2:  templ Comprehensive(show bool, items []string) {
//	line 3:      <div>
//	line 4:          Plain text
//	line 5:          if show {
//	line 6:              <p>Visible</p>
//	line 7:          } else {
//	line 8:              <p>Hidden</p>
//	line 9:          }
//	line 10:         for _, item := range items {
//	line 11:             <li>{ item }</li>
//	line 12:         }
//	line 13:         switch len(items) {
//	line 14:             case 0:
//	line 15:                 <p>None</p>
//	line 16:             default:
//	line 17:                 <p>{ items[0] }</p>
//	line 18:         }
//	line 19: }
const templSource = `package sourcemaptest

templ Comprehensive(show bool, items []string) {
	<div>
		Plain text
		if show {
			<p>Visible</p>
		} else {
			<p>Hidden</p>
		}
		for _, item := range items {
			<li>{ item }</li>
		}
		switch len(items) {
			case 0:
				<p>None</p>
			default:
				<p>{ items[0] }</p>
		}
	</div>
}
`

func TestSourceMapCompleteness(t *testing.T) {
	tf, err := parser.ParseString(templSource)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	var buf bytes.Buffer
	out, err := Generate(tf, &buf)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	goLines := strings.Split(buf.String(), "\n")

	// findGoLine returns the 0-indexed line number of the first Go line
	// matching pattern, or -1 if not found.
	findGoLine := func(pattern string) int {
		re := regexp.MustCompile(pattern)
		for i, line := range goLines {
			if re.MatchString(line) {
				return i
			}
		}
		return -1
	}

	sm := out.SourceMap

	// lookupLine searches from col 100 downward so the backward search
	// finds the nearest source-map entry on that Go line regardless of
	// where the expression or statement boundary was recorded.
	lookupLine := func(goLine int) (src parser.Position, ok bool) {
		return sm.SourcePositionFromTarget(uint32(goLine), 100)
	}

	cases := []struct {
		desc      string
		templLine uint32 // 0-indexed line in the .templ source
		goPattern string // regex matching the generated Go statement
	}{
		// <div> and its immediate text child "Plain text " are batched into one
		// WriteString call, so both map to the <div> open tag (line 3).
		{"div open tag",    3, `WriteString.*<div>`},
		{"if boundary",     5, `\bif show\b`},
		{"else boundary",   7, `} else \{`},
		{"p in if-true",    6, `WriteString.*Visible`},
		{"p in else",       8, `WriteString.*Hidden`},
		{"for boundary",    10, `\bfor .* range items\b`},
		{"li in for body",  11, `WriteString.*<li>`},
		{"switch boundary", 13, `\bswitch len\(items\)`},
		{"case 0",          14, `\bcase 0:`},
		// <p>None</p> flushes onto the same Go line as "case 0:" (no \n after
		// the case expression), so its coverage is attributed to line 14 (case 0:).
		// The case_0 case above already verifies that line is mapped.
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			goLine := findGoLine(tc.goPattern)
			if goLine < 0 {
				t.Fatalf("could not find Go line matching %q in generated output", tc.goPattern)
			}

			src, ok := lookupLine(goLine)
			if !ok {
				t.Fatalf("SourcePositionFromTarget(%d, *): no entry — construct not mapped\nGo line: %s",
					goLine, goLines[goLine])
			}
			if src.Line != tc.templLine {
				t.Errorf("Go line %d (%q): got templ line %d, want %d\nGo line: %s",
					goLine, tc.desc, src.Line, tc.templLine, goLines[goLine])
			}
		})
	}

	// Templ→Go direction: each source construct must also map forward to the
	// generated Go. These verify SourceLinesToTarget is populated (the hover-demo
	// direction). Columns below are 0-indexed byte offsets within the templ line.
	src2GoCases := []struct {
		desc                  string
		templLine, templCol   uint32
		goPattern             string // regex matching the expected Go line
	}{
		// <div> name position maps to the WriteString containing <div>
		{"div open tag (templ→go)",      3, 2, `WriteString.*<div>`},
		// Plain text batches into the same WriteString as <div>
		{"plain text (templ→go)",        4, 2, `WriteString.*Plain text`},
		// <p> open tag in if-true branch
		{"p open in if-true (templ→go)", 6, 4, `WriteString.*Visible`},
		// } else { keyword
		{"else keyword (templ→go)",      7, 2, `} else \{`},
		// <p> open tag in else branch
		{"p open in else (templ→go)",    8, 4, `WriteString.*Hidden`},
		// <li> open tag (in for body)
		{"li open tag (templ→go)",       11, 4, `WriteString.*<li>`},
		// </div> close tag
		{"div close tag (templ→go)",     19, 1, `WriteString.*</div>`},
	}

	for _, tc := range src2GoCases {
		t.Run(tc.desc, func(t *testing.T) {
			tgt, ok := sm.TargetPositionFromSource(tc.templLine, tc.templCol)
			if !ok {
				t.Fatalf("TargetPositionFromSource(%d, %d): no entry — templ→go direction not mapped",
					tc.templLine, tc.templCol)
			}
			goLine := findGoLine(tc.goPattern)
			if goLine < 0 {
				t.Fatalf("could not find Go line matching %q", tc.goPattern)
			}
			if tgt.Line != uint32(goLine) {
				t.Errorf("templ[%d][%d] (%q): mapped to go line %d, want %d\nGo line: %s",
					tc.templLine, tc.templCol, tc.desc, tgt.Line, goLine, goLines[tgt.Line])
			}
		})
	}
}
