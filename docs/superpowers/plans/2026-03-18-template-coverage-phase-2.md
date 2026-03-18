# Template Coverage Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add coverage instrumentation to 7 generator write methods to track if/else branches, switch cases, for loops, template calls, HTML elements, text literals, and children expressions.

**Architecture:** Following Phase 1's pattern, each write method in `generator.go` will check `g.options.Coverage` and emit `templruntime.CoverageTrack(filename, line, col)` calls at appropriate coverage points. Tests verify each coverage point type generates tracking calls and produces valid coverage profiles.

**Tech Stack:** Go, templ generator, templruntime coverage API

---

## File Structure

### Files to Modify

**generator/generator.go** - Add coverage instrumentation to 7 methods:
- `writeIfExpression` (~line 728) - Track if condition, then-branch, else-branch
- `writeSwitchExpression` (~line 792) - Track switch expression, each case
- `writeForExpression` (~line 932) - Track for statement, loop body
- `writeTemplElementExpression` (~line 840) - Track template call site
- `writeElement` (~line 998) - Track opening tag, closing tag
- `writeText` (~line 1720) - Track text literal
- `writeChildrenExpression` (~line 830) - Track children expression

### Files to Create

**generator/test-coverage-if/** - If/else branch coverage tests
- `template.templ` - Template with if/else branches
- `render_test.go` - Test that verifies if/else coverage tracking

**generator/test-coverage-switch/** - Switch case coverage tests
- `template.templ` - Template with switch statement
- `render_test.go` - Test that verifies switch coverage tracking

**generator/test-coverage-for/** - For loop coverage tests
- `template.templ` - Template with for loop
- `render_test.go` - Test that verifies for loop coverage tracking

**generator/test-coverage-call/** - Template call coverage tests
- `template.templ` - Template that calls another template
- `helper.templ` - Helper template being called
- `render_test.go` - Test that verifies call site coverage tracking

**generator/test-coverage-element/** - HTML element coverage tests
- `template.templ` - Template with regular and void elements
- `render_test.go` - Test that verifies element coverage tracking

**generator/test-coverage-text/** - Text literal coverage tests
- `template.templ` - Template with text literals
- `render_test.go` - Test that verifies text coverage tracking

**generator/test-coverage-children/** - Children expression coverage tests
- `template.templ` - Template with children expression
- `render_test.go` - Test that verifies children coverage tracking

---

## Task 1: Instrument writeIfExpression (If/Else Branches)

**Files:**
- Modify: `generator/generator.go:728-790`
- Create: `generator/test-coverage-if/template.templ`
- Create: `generator/test-coverage-if/render_test.go`

- [ ] **Step 1: Create test template**

```bash
mkdir -p generator/test-coverage-if
```

Create `generator/test-coverage-if/template.templ`:
```templ
package testcoverageif

templ WithIf(show bool) {
	if show {
		<div>Shown</div>
	}
}

templ WithIfElse(show bool) {
	if show {
		<div>Then branch</div>
	} else {
		<div>Else branch</div>
	}
}
```

- [ ] **Step 2: Generate without coverage to create baseline**

```bash
cd generator/test-coverage-if
templ generate
```

Expected: Creates `template_templ.go` without CoverageTrack calls

- [ ] **Step 3: Create test that will verify coverage**

Create `generator/test-coverage-if/render_test.go`:
```go
package testcoverageif

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestIfCoverage(t *testing.T) {
	// Setup coverage directory
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	// Render both templates
	ctx := context.Background()
	var buf strings.Builder

	// Test if-then (show=true)
	if err := WithIf(true).Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	// Test if-then-else both paths
	buf.Reset()
	if err := WithIfElse(true).Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := WithIfElse(false).Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	// Flush and verify coverage
	if err := templruntime.FlushCoverage(); err != nil {
		t.Fatal(err)
	}

	// Read coverage profile
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

	// Verify we have coverage points for if/else
	points := profile.Files["generator/test-coverage-if/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points for template.templ")
	}

	// Should have points for:
	// - if condition (line 4)
	// - then branch (line 5)
	// - if condition (line 10)
	// - then branch (line 11)
	// - else branch (line 13)
	if len(points) < 5 {
		t.Errorf("expected at least 5 coverage points, got %d", len(points))
	}
}
```

- [ ] **Step 4: Add missing import**

Add to imports in `render_test.go`:
```go
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)
```

- [ ] **Step 5: Run test to verify it fails**

```bash
cd generator/test-coverage-if
go test -v
```

Expected: FAIL - no coverage points found (instrumentation not added yet)

- [ ] **Step 6: Add nodeRange helper function**

The `parser.Node` interface doesn't expose a `Range()` method, but all concrete node types have a `Range` field. Add a helper function in `generator/generator.go` to extract Range from any Node:

```go
// nodeRange extracts the Range from a Node.
// All concrete Node types have a Range field but the interface doesn't expose it.
func nodeRange(n parser.Node) parser.Range {
	switch n := n.(type) {
	case *parser.Element:
		return n.Range
	case *parser.Text:
		return n.Range
	case *parser.IfExpression:
		return n.Range
	case *parser.SwitchExpression:
		return n.Range
	case *parser.ForExpression:
		return n.Range
	case *parser.StringExpression:
		return n.Range
	case *parser.TemplElementExpression:
		return n.Range
	case *parser.ChildrenExpression:
		return n.Range
	case *parser.GoCode:
		return n.Range
	case *parser.RawElement:
		return n.Range
	case *parser.ScriptElement:
		return n.Range
	case *parser.HTMLComment:
		return n.Range
	case *parser.GoComment:
		return n.Range
	default:
		return parser.Range{}
	}
}
```

- [ ] **Step 7: Add instrumentation to writeIfExpression**

In `generator/generator.go`, find `writeIfExpression` method (~line 728).

Add coverage tracking before writing the if condition check. Find the section that writes the if statement and add before it:

```go
// Emit coverage tracking for if condition
if g.options.Coverage {
	line := n.Range.From.Line
	col := n.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

Add coverage tracking at the start of the then-branch. In `writeIfExpression`, find where `g.writeNodes(indentLevel, stripLeadingAndTrailingWhitespace(n.Then), ...)` is called (line ~745). Add tracking just before that call, inside the indented block. Use the `nodeRange` helper to get the position of the first child node:

```go
// Emit coverage tracking for then-branch entry
if g.options.Coverage && len(n.Then) > 0 {
	r := nodeRange(n.Then[0])
	if r.From.Line > 0 {
		trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
			g.options.FileName, r.From.Line, r.From.Col)
		if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
			return err
		}
	}
}
```

Add coverage tracking at the start of else-branch (if present). Find where `len(n.Else) > 0` is checked (~line 772) and the else body is written. Add tracking inside the else block before `g.writeNodes`:

```go
// Emit coverage tracking for else-branch entry
if g.options.Coverage && len(n.Else) > 0 {
	r := nodeRange(n.Else[0])
	if r.From.Line > 0 {
		trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
			g.options.FileName, r.From.Line, r.From.Col)
		if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
			return err
		}
	}
}
```

Also add coverage tracking for each `else if` branch. Inside the `for _, elseIf := range n.ElseIfs` loop (~line 750), add tracking before `g.writeNodes(indentLevel, stripLeadingAndTrailingWhitespace(elseIf.Then), ...)`. Use `elseIf.Range.From` for the position since `ElseIfExpression` has its own Range:

```go
// Emit coverage tracking for else-if branch entry
if g.options.Coverage {
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		g.options.FileName, elseIf.Range.From.Line, elseIf.Range.From.Col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

- [ ] **Step 8: Regenerate test template with coverage**

```bash
cd generator/test-coverage-if
templ generate --coverage
```

Expected: `template_templ.go` now contains CoverageTrack calls

- [ ] **Step 9: Run test to verify it passes**

```bash
cd generator/test-coverage-if
go test -v
```

Expected: PASS - coverage points are tracked

- [ ] **Step 10: Verify generated code has tracking calls**

```bash
grep -n "CoverageTrack" generator/test-coverage-if/template_templ.go
```

Expected: Multiple lines showing CoverageTrack calls at if conditions and branches

- [ ] **Step 11: Commit**

```bash
git add generator/generator.go generator/test-coverage-if/
git commit -m "$(cat <<'EOF'
feat(coverage): add instrumentation for if/else branches

Instrument writeIfExpression to track:
- If condition evaluation
- Then-branch entry
- Else-branch entry (if present)

Adds test-coverage-if directory with test template and coverage verification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Instrument writeSwitchExpression (Switch Cases)

**Files:**
- Modify: `generator/generator.go:792-900`
- Create: `generator/test-coverage-switch/template.templ`
- Create: `generator/test-coverage-switch/render_test.go`

- [ ] **Step 1: Create test template**

```bash
mkdir -p generator/test-coverage-switch
```

Create `generator/test-coverage-switch/template.templ`:
```templ
package testcoverageswitch

templ WithSwitch(value string) {
	switch value {
	case "a":
		<div>Case A</div>
	case "b":
		<div>Case B</div>
	default:
		<div>Default</div>
	}
}
```

- [ ] **Step 2: Create coverage test**

Create `generator/test-coverage-switch/render_test.go`:
```go
package testcoverageswitch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestSwitchCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	// Execute all cases
	for _, val := range []string{"a", "b", "other"} {
		buf.Reset()
		if err := WithSwitch(val).Render(ctx, &buf); err != nil {
			t.Fatal(err)
		}
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

	points := profile.Files["generator/test-coverage-switch/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	// Should have points for switch expression + 3 cases
	if len(points) < 4 {
		t.Errorf("expected at least 4 coverage points, got %d", len(points))
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd generator/test-coverage-switch
go test -v
```

Expected: FAIL - no coverage points

- [ ] **Step 4: Add instrumentation to writeSwitchExpression**

In `generator/generator.go`, find `writeSwitchExpression` (~line 792).

Add coverage tracking before the switch statement:

```go
// Emit coverage tracking for switch expression
if g.options.Coverage {
	line := n.Range.From.Line
	col := n.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

Add coverage tracking at the start of each case block. Inside the `for _, c := range n.Cases` loop (~line 809), after writing the case expression and before `g.writeNodes(indentLevel, stripLeadingAndTrailingWhitespace(c.Children), ...)`:

```go
// Emit coverage tracking for case entry
if g.options.Coverage {
	line := c.Expression.Range.From.Line
	col := c.Expression.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

- [ ] **Step 5: Regenerate and test**

```bash
cd generator/test-coverage-switch
templ generate --coverage
go test -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add generator/generator.go generator/test-coverage-switch/
git commit -m "$(cat <<'EOF'
feat(coverage): add instrumentation for switch cases

Instrument writeSwitchExpression to track:
- Switch expression evaluation
- Entry to each case block
- Default case entry

Adds test-coverage-switch directory with coverage verification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Instrument writeForExpression (For Loops)

**Files:**
- Modify: `generator/generator.go:932-1050`
- Create: `generator/test-coverage-for/template.templ`
- Create: `generator/test-coverage-for/render_test.go`

- [ ] **Step 1: Create test template**

```bash
mkdir -p generator/test-coverage-for
```

Create `generator/test-coverage-for/template.templ`:
```templ
package testcoveragefor

templ WithFor(items []string) {
	for _, item := range items {
		<div>{ item }</div>
	}
}

templ WithEmptyFor() {
	for _, item := range []string{} {
		<div>{ item }</div>
	}
}
```

- [ ] **Step 2: Create coverage test**

Create `generator/test-coverage-for/render_test.go`:
```go
package testcoveragefor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestForCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	// Test with items (loop executes)
	if err := WithFor([]string{"a", "b"}).Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	// Test without items (loop doesn't execute body)
	buf.Reset()
	if err := WithEmptyFor().Render(ctx, &buf); err != nil {
		t.Fatal(err)
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

	points := profile.Files["generator/test-coverage-for/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	// Should have points for:
	// - for statement (line 4)
	// - loop body (line 5) - hit 2 times
	// - for statement (line 10)
	// - loop body (line 11) - hit 0 times
	if len(points) < 3 {
		t.Errorf("expected at least 3 coverage points, got %d", len(points))
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd generator/test-coverage-for
go test -v
```

Expected: FAIL - no coverage points

- [ ] **Step 4: Add instrumentation to writeForExpression**

In `generator/generator.go`, find `writeForExpression` (~line 932).

Add coverage tracking before the for statement:

```go
// Emit coverage tracking for for statement
if g.options.Coverage {
	line := n.Range.From.Line
	col := n.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

Add coverage tracking at the start of loop body. In `writeForExpression`, find where `g.writeNodes(indentLevel, stripLeadingAndTrailingWhitespace(n.Children), ...)` is called (~line 949). Add tracking just before that call, inside the indented block. Use the `nodeRange` helper:

```go
// Emit coverage tracking for loop body entry
if g.options.Coverage && len(n.Children) > 0 {
	r := nodeRange(n.Children[0])
	if r.From.Line > 0 {
		trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
			g.options.FileName, r.From.Line, r.From.Col)
		if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
			return err
		}
	}
}
```

- [ ] **Step 5: Regenerate and test**

```bash
cd generator/test-coverage-for
templ generate --coverage
go test -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add generator/generator.go generator/test-coverage-for/
git commit -m "$(cat <<'EOF'
feat(coverage): add instrumentation for for loops

Instrument writeForExpression to track:
- For statement evaluation
- Loop body entry (distinguishes 0 iterations from N iterations)

Adds test-coverage-for directory with coverage verification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Instrument writeTemplElementExpression (Template Calls)

**Files:**
- Modify: `generator/generator.go:840-865`
- Create: `generator/test-coverage-call/template.templ`
- Create: `generator/test-coverage-call/helper.templ`
- Create: `generator/test-coverage-call/render_test.go`

- [ ] **Step 1: Create test templates**

```bash
mkdir -p generator/test-coverage-call
```

Create `generator/test-coverage-call/helper.templ`:
```templ
package testcoveragecall

templ Helper(text string) {
	<span>{ text }</span>
}
```

Create `generator/test-coverage-call/template.templ`:
```templ
package testcoveragecall

templ CallsHelper() {
	<div>
		@Helper("test")
	</div>
}
```

- [ ] **Step 2: Create coverage test**

Create `generator/test-coverage-call/render_test.go`:
```go
package testcoveragecall

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestCallCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	if err := CallsHelper().Render(ctx, &buf); err != nil {
		t.Fatal(err)
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

	// Should have coverage for the call site in template.templ
	points := profile.Files["generator/test-coverage-call/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points for call site")
	}

	// Should also have coverage inside Helper
	helperPoints := profile.Files["generator/test-coverage-call/helper.templ"]
	if len(helperPoints) == 0 {
		t.Fatal("expected coverage points in helper template")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd generator/test-coverage-call
go test -v
```

Expected: FAIL - no coverage for call site

- [ ] **Step 4: Add instrumentation to writeTemplElementExpression**

In `generator/generator.go`, find `writeTemplElementExpression` (~line 840).

Add coverage tracking before the Render() call:

```go
// Emit coverage tracking for template call site
if g.options.Coverage {
	line := n.Range.From.Line
	col := n.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

- [ ] **Step 5: Regenerate and test**

```bash
cd generator/test-coverage-call
templ generate --coverage
go test -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add generator/generator.go generator/test-coverage-call/
git commit -m "$(cat <<'EOF'
feat(coverage): add instrumentation for template calls

Instrument writeTemplElementExpression to track:
- Template call sites (where templates invoke other templates)

Called templates have their own internal coverage tracking.

Adds test-coverage-call directory with coverage verification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Instrument writeElement (HTML Elements)

**Files:**
- Modify: `generator/generator.go:998-1180`
- Create: `generator/test-coverage-element/template.templ`
- Create: `generator/test-coverage-element/render_test.go`

- [ ] **Step 1: Create test template**

```bash
mkdir -p generator/test-coverage-element
```

Create `generator/test-coverage-element/template.templ`:
```templ
package testcoverageelement

templ WithElements() {
	<div class="container">
		<button type="button">Click</button>
		<input type="text" />
		<img src="test.png" />
	</div>
}
```

- [ ] **Step 2: Create coverage test**

Create `generator/test-coverage-element/render_test.go`:
```go
package testcoverageelement

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestElementCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	if err := WithElements().Render(ctx, &buf); err != nil {
		t.Fatal(err)
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

	points := profile.Files["generator/test-coverage-element/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	// Should have points for:
	// - div opening, div closing
	// - button opening, button closing
	// - input opening (void element, no closing)
	// - img opening (void element, no closing)
	// Minimum: 6 points (3 elements * 2 tags, but void elements only 1)
	if len(points) < 6 {
		t.Errorf("expected at least 6 coverage points, got %d", len(points))
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd generator/test-coverage-element
go test -v
```

Expected: FAIL - no coverage points

- [ ] **Step 4: Add instrumentation to writeElement**

In `generator/generator.go`, find `writeElement` (~line 998).

Add coverage tracking before writing opening tag:

```go
// Emit coverage tracking for opening tag
if g.options.Coverage {
	line := n.Range.From.Line
	col := n.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

Add coverage tracking before writing closing tag (skip for void elements). In `writeElement`, find where the closing tag is written (~line 1034, `fmt.Sprintf("</%s>"...)`). Add tracking before that line. Note: the existing code at line 1027 already skips children and close tag for void elements via `if n.IsVoidElement() && len(n.Children) == 0 { return nil }`, so the closing tag tracking will naturally only execute for non-void elements:

```go
// Emit coverage tracking for closing tag
if g.options.Coverage {
	// Use Range.To for closing tag position
	line := n.Range.To.Line
	col := n.Range.To.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

- [ ] **Step 5: Regenerate and test**

```bash
cd generator/test-coverage-element
templ generate --coverage
go test -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add generator/generator.go generator/test-coverage-element/
git commit -m "$(cat <<'EOF'
feat(coverage): add instrumentation for HTML elements

Instrument writeElement to track:
- Opening tag emission
- Closing tag emission (for non-void elements)

Helps identify incomplete renders in streaming scenarios.

Adds test-coverage-element directory with coverage verification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Instrument writeText (Text Literals)

**Files:**
- Modify: `generator/generator.go:1720-1750`
- Create: `generator/test-coverage-text/template.templ`
- Create: `generator/test-coverage-text/render_test.go`

- [ ] **Step 1: Create test template**

```bash
mkdir -p generator/test-coverage-text
```

Create `generator/test-coverage-text/template.templ`:
```templ
package testcoveragetext

templ WithText() {
	<div>
		Plain text here
		<span>More text</span>
		Another line
	</div>
}
```

- [ ] **Step 2: Create coverage test**

Create `generator/test-coverage-text/render_test.go`:
```go
package testcoveragetext

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestTextCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	if err := WithText().Render(ctx, &buf); err != nil {
		t.Fatal(err)
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

	points := profile.Files["generator/test-coverage-text/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	// Should have coverage for text literals
	if len(points) < 3 {
		t.Errorf("expected at least 3 coverage points, got %d", len(points))
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd generator/test-coverage-text
go test -v
```

Expected: FAIL - no coverage points

- [ ] **Step 4: Add instrumentation to writeText**

In `generator/generator.go`, find `writeText` (~line 1720).

Add coverage tracking before writing text:

```go
// Emit coverage tracking for text literal
if g.options.Coverage {
	line := n.Range.From.Line
	col := n.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

- [ ] **Step 5: Regenerate and test**

```bash
cd generator/test-coverage-text
templ generate --coverage
go test -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add generator/generator.go generator/test-coverage-text/
git commit -m "$(cat <<'EOF'
feat(coverage): add instrumentation for text literals

Instrument writeText to track:
- Static text node rendering

Provides complete picture of executed template sections.

Adds test-coverage-text directory with coverage verification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Instrument writeChildrenExpression (Children Blocks)

**Files:**
- Modify: `generator/generator.go:830-838`
- Create: `generator/test-coverage-children/template.templ`
- Create: `generator/test-coverage-children/render_test.go`

- [ ] **Step 1: Create test template**

```bash
mkdir -p generator/test-coverage-children
```

Create `generator/test-coverage-children/template.templ`:
```templ
package testcoveragechildren

templ Wrapper() {
	<div class="wrapper">
		{ children... }
	</div>
}

templ UsesWrapper() {
	@Wrapper() {
		<span>Child content</span>
	}
}
```

- [ ] **Step 2: Create coverage test**

Create `generator/test-coverage-children/render_test.go`:
```go
package testcoveragechildren

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestChildrenCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	if err := UsesWrapper().Render(ctx, &buf); err != nil {
		t.Fatal(err)
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

	points := profile.Files["generator/test-coverage-children/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points")
	}

	// Should have coverage for children expression
	hasChildrenPoint := false
	for _, point := range points {
		// Children expression is on line 5
		if point.Line == 5 {
			hasChildrenPoint = true
			break
		}
	}
	if !hasChildrenPoint {
		t.Error("expected coverage point for children expression")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd generator/test-coverage-children
go test -v
```

Expected: FAIL - no coverage for children expression

- [ ] **Step 4: Add instrumentation to writeChildrenExpression**

In `generator/generator.go`, find `writeChildrenExpression` (~line 830).

First, update the method signature to accept the node for Range access. Change:
```go
func (g *generator) writeChildrenExpression(indentLevel int) (err error) {
```
to:
```go
func (g *generator) writeChildrenExpression(indentLevel int, n *parser.ChildrenExpression) (err error) {
```

Then update the call site in the `writeNodes` switch statement (search for `case *parser.ChildrenExpression:`):
```go
case *parser.ChildrenExpression:
	err = g.writeChildrenExpression(indentLevel, n)
```

Then add coverage tracking before rendering children:

```go
// Emit coverage tracking for children expression
if g.options.Coverage {
	line := n.Range.From.Line
	col := n.Range.From.Col
	filename := g.options.FileName
	trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
		filename, line, col)
	if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
		return err
	}
}
```

- [ ] **Step 5: Regenerate and test**

```bash
cd generator/test-coverage-children
templ generate --coverage
go test -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add generator/generator.go generator/test-coverage-children/
git commit -m "$(cat <<'EOF'
feat(coverage): add instrumentation for children expressions

Instrument writeChildrenExpression to track:
- Children block rendering

Completes Phase 2 coverage instrumentation.

Adds test-coverage-children directory with coverage verification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Update Documentation

**Files:**
- Modify: `docs/docs/09-developer-tools/07-coverage.md`

- [ ] **Step 1: Update coverage documentation**

Update `docs/docs/09-developer-tools/07-coverage.md` to reflect Phase 2 completion.

In the "Coverage Points" section (lines 32-41), update to show all tracked points:

```markdown
## Coverage Points

Templ tracks execution at the expression and element level:

- String expressions: `{ name }`
- If/else branches: condition, then-branch, else-branch
- Switch cases: expression, each case, default
- For loops: statement, body entry
- Template calls: `@component()`
- Static HTML elements: opening and closing tags
- Text literals
- Children expressions: `{ children... }`
```

In the "Limitations" section (lines 118-122), update:

```markdown
## Limitations

- CSS and script templates not yet supported (future enhancement)
- Requires regenerating templates with `--coverage` flag
- Profile format is v1 (may evolve in future releases)
```

- [ ] **Step 2: Verify documentation renders correctly**

```bash
# If there's a doc preview command, run it
# Otherwise just verify markdown syntax
cat docs/docs/09-developer-tools/07-coverage.md
```

- [ ] **Step 3: Commit documentation update**

```bash
git add docs/docs/09-developer-tools/07-coverage.md
git commit -m "$(cat <<'EOF'
docs: update coverage documentation for Phase 2

Document all coverage points now tracked:
- If/else branches
- Switch cases
- For loops
- Template calls
- HTML elements
- Text literals
- Children expressions

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Integration Test

**Files:**
- Create: `generator/test-coverage-integration/template.templ`
- Create: `generator/test-coverage-integration/render_test.go`

- [ ] **Step 1: Create comprehensive integration test template**

```bash
mkdir -p generator/test-coverage-integration
```

Create `generator/test-coverage-integration/template.templ`:
```templ
package testcoverageintegration

templ Helper(msg string) {
	<em>{ msg }</em>
}

templ Comprehensive(show bool, items []string) {
	<div>
		Plain text
		{ "string expression" }

		if show {
			<p>If branch</p>
		} else {
			<p>Else branch</p>
		}

		switch len(items) {
		case 0:
			<p>Empty</p>
		case 1:
			<p>One</p>
		default:
			<p>Many</p>
		}

		for _, item := range items {
			<li>{ item }</li>
		}

		@Helper("called")

		<input type="text" />
	</div>
}

templ WithChildren() {
	<section>
		{ children... }
	</section>
}

templ Main() {
	@WithChildren() {
		<span>Child content</span>
	}
}
```

- [ ] **Step 2: Create integration test**

Create `generator/test-coverage-integration/render_test.go`:
```go
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
		name  string
		comp  templ.Component
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

	// Verify we have comprehensive coverage
	// Should have points for:
	// - Text, string expression
	// - If condition, then, else
	// - Switch expression, 3 cases
	// - For statement, loop body
	// - Template call
	// - Element tags
	// - Children expression
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
```

- [ ] **Step 3: Run integration test**

```bash
cd generator/test-coverage-integration
templ generate --coverage
go test -v
```

Expected: PASS - all coverage points tracked

- [ ] **Step 4: Commit integration test**

```bash
git add generator/test-coverage-integration/
git commit -m "$(cat <<'EOF'
test(coverage): add comprehensive integration test

Tests all Phase 2 coverage points in a single template:
- String expressions
- If/else branches
- Switch cases
- For loops
- Template calls
- HTML elements
- Text literals
- Children expressions

Verifies complete coverage instrumentation.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Regression Test - Verify Phase 1 Still Works

**Files:**
- Test: `generator/test-script-usage/`

- [ ] **Step 1: Regenerate Phase 1 test with coverage**

```bash
cd generator/test-script-usage
templ generate --coverage
```

- [ ] **Step 2: Run existing tests**

```bash
cd generator/test-script-usage
go test -v
```

Expected: All tests PASS - no regression from Phase 1

- [ ] **Step 3: Verify string expressions still tracked**

```bash
grep -c "CoverageTrack" generator/test-script-usage/template_templ.go
```

Expected: Multiple CoverageTrack calls present

- [ ] **Step 4: Create regression test**

Create `generator/test-script-usage/coverage_test.go`:
```go
package testscriptusage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
)

func TestPhase1RegressionCoverage(t *testing.T) {
	coverDir := t.TempDir()
	t.Setenv("TEMPLCOVERDIR", coverDir)
	templruntime.EnableCoverageForTesting()
	defer templruntime.FlushCoverage()

	ctx := context.Background()
	var buf strings.Builder

	// Render template with string expressions (Phase 1 feature)
	if err := Button("test").Render(ctx, &buf); err != nil {
		t.Fatal(err)
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

	points := profile.Files["generator/test-script-usage/template.templ"]
	if len(points) == 0 {
		t.Fatal("expected coverage points - Phase 1 regression")
	}

	// Phase 1 tracked string expressions
	// Verify they're still being tracked
	t.Logf("Phase 1 coverage points: %d", len(points))
}
```

- [ ] **Step 5: Run regression test**

```bash
cd generator/test-script-usage
go test -v -run TestPhase1Regression
```

Expected: PASS

- [ ] **Step 6: Commit regression test**

```bash
git add generator/test-script-usage/coverage_test.go
git commit -m "$(cat <<'EOF'
test(coverage): add Phase 1 regression test

Verify string expression coverage (Phase 1) still works after
Phase 2 additions.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Success Criteria

- [ ] All 7 write methods instrumented with coverage tracking
- [ ] Each coverage point type has dedicated test directory
- [ ] Integration test exercises all coverage points
- [ ] Phase 1 regression test passes
- [ ] Documentation updated
- [ ] All tests pass with `--coverage` flag
- [ ] Generated code compiles and runs correctly
- [ ] Coverage profiles contain expected points with correct hit counts
