# Template Coverage Metrics Design

**Date:** 2026-03-17
**Status:** Approved
**Author:** AI Assistant + User

## Overview

This design introduces coverage tracking for templ HTML templates, enabling developers to measure which parts of their templates execute during tests and identify untested code paths.

## Goals

- Track expression-level coverage in `.templ` files during test execution
- Generate human-readable and machine-parseable coverage reports
- Enable CI/CD threshold enforcement
- Support AI agents in identifying gaps and adding missing tests
- Maintain zero runtime overhead in production builds

## Non-Goals

- CSS/script template coverage (future enhancement)
- Real-time coverage updates during development
- Integration with Go's native coverage tooling (attempted but source map reversal proved too complex)

## Architecture

The system consists of four independent layers:

### 1. Generator (Instrumentation)

**Entry Point:** `templ generate --coverage` flag

**Behavior:**
- Generator emits `templruntime.CoverageTrack(filename, line, col)` calls before each trackable coverage point
- Uses parser `Range` fields (not SourceMap) for source positions
- Filenames stored as strings (not hashes) for readability

**Coverage Points Tracked:**
- String expressions: `{ name }`, `{ person.Name() }`
- If/else branches: condition + each branch body
- Switch cases: switch expression + each case body
- For loops: loop expression + body
- Template calls: `@component(data)`, `@otherTemplate()`
- Children expressions: `{ children... }`
- Static HTML elements: `<div>`, `<p>`
- Text literals: `Hello, world!`

**Not Tracked (v1):**
- Whitespace (formatting only, creates noise)
- Comments (low semantic value)
- CSS/script templates (future priority)

**Example Instrumented Code:**

Before (normal generation):
```go
var templ_var string
templ_var, err = templ.JoinStringErrs(name)
_, err = buffer.WriteString(templ.EscapeString(templ_var))
```

After (coverage mode):
```go
templruntime.CoverageTrack("template.templ", 5, 10)
var templ_var string
templ_var, err = templ.JoinStringErrs(name)
_, err = buffer.WriteString(templ.EscapeString(templ_var))
```

### 2. Runtime (Collection)

**Location:** `runtime/coverage.go` (new file in existing runtime package)

**Data Structure:**
```go
type CoverageRegistry struct {
    mu    sync.Mutex
    files map[string]map[Position]uint32  // filename → position → hit count
}

type Position struct {
    Line uint32
    Col  uint32
}
```

**Activation:**
- Only initializes if `TEMPLCOVERDIR` environment variable is set
- Global registry: `var coverageRegistry *CoverageRegistry`
- Thread-safe (mutex-protected)

**Tracking API:**
```go
// Called by generated code
func CoverageTrack(filename string, line, col uint32) {
    if coverageRegistry == nil {
        return  // No-op if coverage disabled (zero overhead)
    }
    coverageRegistry.Record(filename, line, col)
}
```

**Profile Writing:**
- Registers exit hook via `init()` to auto-flush on process termination
- Writes to `$TEMPLCOVERDIR/templ-<pid>-<timestamp>.json`
- Unique filenames prevent conflicts between concurrent test processes

**Environment Variables:**
- `TEMPLCOVERDIR` — directory to write coverage profiles (mirrors Go's `GOCOVERDIR`)

### 3. Profile Format (Storage)

**Format:** JSON for human/AI readability

**Schema:**
```json
{
  "version": "1",
  "mode": "count",
  "files": {
    "templates/user/profile.templ": [
      {
        "line": 5,
        "col": 3,
        "hits": 12,
        "type": "expression"
      },
      {
        "line": 7,
        "col": 10,
        "hits": 0,
        "type": "element"
      },
      {
        "line": 10,
        "col": 5,
        "hits": 8,
        "type": "branch"
      }
    ]
  }
}
```

**Fields:**
- `version` — schema version for future evolution
- `mode` — "count" (hit counts) vs "set" (covered/not covered flag)
- `type` — expression/element/branch/text (aids filtering/reporting)

**Directory-Based Workflow:**

Mirrors Go's `GOCOVERDIR` pattern:
```bash
# Unit tests
TEMPLCOVERDIR=coverage/unit go test ./...

# Integration tests
TEMPLCOVERDIR=coverage/integration go test ./integration/...

# Merge all profiles
templ coverage merge -i=coverage/unit,coverage/integration -o=coverage.json
```

### 4. Tooling (Analysis & Reporting)

**New CLI Subcommand:** `templ coverage`

#### `templ coverage merge`

Combines multiple profile files into one.

```bash
templ coverage merge -i=coverage/unit,coverage/integration -o=merged.json
templ coverage merge -i=coverage/* -o=merged.json  # glob support
```

**Algorithm:**
- Load all input profiles
- For each coverage point, sum hit counts across profiles
- Output single merged profile with combined data

#### `templ coverage report`

Generates coverage reports in multiple formats.

```bash
# Default: terminal output
templ coverage report -i=merged.json

# HTML report (writes to coverage.html by default)
templ coverage report -i=merged.json --html
templ coverage report -i=merged.json --html -o=report.html

# Machine-readable JSON for AI agents
templ coverage report -i=merged.json --json
```

**Terminal Output:**
```
Template Coverage Report
========================
templates/user/profile.templ    85.7%  (12/14 expressions)
templates/nav.templ            100.0%  (8/8 expressions)
templates/footer.templ          66.7%  (4/6 expressions)
------------------------
TOTAL                           84.0%  (24/28 expressions)
```

**HTML Output:**
- Annotated source view (like `go tool cover -html`)
- Green highlighting for covered lines
- Red highlighting for uncovered lines
- Sidebar with per-file percentages

**JSON Output:**
```json
{
  "total": 84.0,
  "files": [
    {
      "path": "templates/user/profile.templ",
      "coverage": 85.7,
      "covered": 12,
      "total": 14,
      "uncovered": [
        {"line": 7, "col": 10, "type": "element"},
        {"line": 15, "col": 5, "type": "branch"}
      ]
    }
  ]
}
```

#### `templ coverage check`

Enforces coverage thresholds for CI/CD.

```bash
templ coverage check -i=merged.json --threshold=80
templ coverage check -i=merged.json --config=.templ-coverage.json
```

**Behavior:**
- Exits 0 if coverage meets threshold
- Exits 1 if coverage below threshold (fails CI build)

**Output on Failure:**
```
Coverage check: FAILED
Overall: 76.5% (threshold: 80.0%)

Files below threshold:
  templates/error.templ: 50.0% (6/12 expressions)
  templates/admin.templ: 62.5% (5/8 expressions)
```

**Configuration File (.templ-coverage.json):**
```json
{
  "threshold": 80.0,
  "perPackageThresholds": {
    "templates/critical/": 95.0,
    "templates/experimental/": 50.0
  },
  "exclude": [
    "templates/generated/*",
    "templates/deprecated/*"
  ]
}
```

CLI flags override config file values.

## Error Handling

### Coverage Generation Failures

1. **Parser errors** (template has syntax errors)
   - Fail generation (same as normal `templ generate`)

2. **Range field missing/invalid** (parser node missing position)
   - Skip that coverage point
   - Log warning: `WARN: Skipping coverage for expression at unknown position`
   - Don't fail entire generation

3. **File ID conflicts** (unlikely with full paths)
   - Use absolute path to disambiguate

### Runtime Collection Failures

1. **TEMPLCOVERDIR doesn't exist**
   - Create directory automatically
   - If can't create: log error to stderr, continue without writing (don't crash tests)

2. **Disk full / write permission denied**
   - Log error to stderr, continue test execution
   - Coverage lost for that run, but tests don't fail

3. **Concurrent test processes**
   - Unique filenames prevent collisions (`templ-<pid>-<timestamp>.json`)
   - Each process writes own profile

4. **Registry lock contention** (high-concurrency tests)
   - Mutex protects map
   - May slow tests slightly (trade-off: correctness over performance)

### Profile Merging Failures

1. **Invalid JSON**
   - Skip corrupted file
   - Log error, continue with remaining profiles

2. **Version mismatch** (old profile format)
   - Error with clear message: `ERROR: Profile version 0 not supported, regenerate with templ v0.x.x`

3. **Empty coverage directory**
   - Error: `ERROR: No coverage profiles found in coverage/`

## Edge Cases

### Multi-line Expressions

```templ
templ example() {
    {
        someVeryLongFunction(
            arg1, arg2, arg3
        )
    }
}
```

Track using `Range.From` (first line of expression). Hit count applies to entire expression, not per-line.

### Templates Calling Templates

```templ
templ parent() {
    @child()
}
```

Both `parent()` entry and `@child()` call are tracked. Coverage shows which templates were called, not just defined.

### Templates Never Called

Coverage report shows 0% (no coverage points hit). Distinguishes "uncovered" from "partially covered".

## Testing Strategy

### Unit Tests

**Generator instrumentation:**
```go
func TestCoverageInstrumentation(t *testing.T) {
    template := parseTemplate(`templ example() { if true { "yes" } }`)
    var buf bytes.Buffer
    Generate(template, &buf, WithCoverage())

    output := buf.String()
    assert.Contains(t, output, `templruntime.CoverageTrack("example.templ", 3, 16)`)
}
```

**Runtime registry:**
```go
func TestCoverageRegistry(t *testing.T) {
    reg := &CoverageRegistry{files: make(map[string]map[Position]uint32)}
    reg.Record("test.templ", 5, 10)
    reg.Record("test.templ", 5, 10)  // Same position twice

    assert.Equal(t, 2, reg.files["test.templ"][Position{5, 10}])
}
```

### Integration Tests

End-to-end workflow validation:
```go
func TestCoverageWorkflow(t *testing.T) {
    // 1. Generate coverage-instrumented template
    // 2. Run template in test mode
    // 3. Flush coverage
    // 4. Verify profile written with expected data
}
```

### Golden Tests

Use existing `generator/test-*` structure:
- `test-coverage-if` — if/else branch coverage
- `test-coverage-for` — loop coverage
- `test-coverage-nested` — nested templates
- `test-coverage-uncovered` — verify 0 hits for uncovered code

### Range Validation Tests

Verify parser Range fields are accurate:
```go
func TestParserRangesAreAccurate(t *testing.T) {
    source := `templ example() { if x { "yes" } }`
    tf := parse(source)
    ifExpr := tf.Nodes[0].(*IfExpression)

    extracted := source[ifExpr.Range.From.Index:ifExpr.Range.To.Index]
    assert.Equal(t, `if x { "yes" }`, extracted)
}
```

Run for every node type to build confidence in Range fields.

### CI Integration

Add to existing `test-cover` workflow:
```bash
# Generate coverage-instrumented templates
templ generate --coverage

# Run tests with coverage collection
TEMPLCOVERDIR=coverage/unit go test ./...

# Merge profiles
templ coverage merge -i=coverage/unit -o=coverage.json

# Generate reports
templ coverage report -i=coverage.json

# Enforce threshold (fails build if below)
templ coverage check -i=coverage.json --threshold=80
```

## Implementation Phases

### Phase 1: Core Infrastructure
1. Add `--coverage` flag to generator
2. Implement basic instrumentation for string expressions only
3. Build runtime registry and profile writing
4. Create simple merge tool

**Deliverable:** Basic coverage for expressions, can write and merge profiles

### Phase 2: Complete Coverage Points
1. Add instrumentation for if/else, for, switch
2. Add instrumentation for elements and text
3. Add template call tracking

**Deliverable:** Full expression and element coverage

### Phase 3: Reporting
1. Build terminal report generator
2. Build HTML report generator
3. Build JSON output format
4. Add threshold checking

**Deliverable:** Complete tooling suite

### Phase 4: Polish & Documentation
1. Comprehensive tests (unit, integration, golden)
2. Range validation tests
3. Documentation and examples
4. CI/CD integration examples

**Deliverable:** Production-ready coverage system

## Risks & Mitigations

### Risk: Parser Range Fields Incomplete

**Likelihood:** Medium
**Impact:** High (coverage would miss code)

**Mitigation:**
- Add comprehensive Range validation tests early
- If gaps found, fix in parser (benefits error reporting too)
- Start with expression-only coverage, expand incrementally

### Risk: Performance Impact on Tests

**Likelihood:** Low
**Impact:** Medium (slower test runs)

**Mitigation:**
- Mutex-based tracking is fast (nanoseconds per call)
- Only active when TEMPLCOVERDIR set
- Zero overhead in production
- Can optimize later if needed (lockless data structures)

### Risk: Profile Format Evolution

**Likelihood:** High (will need to change over time)
**Impact:** Low (manageable with versioning)

**Mitigation:**
- Version field in profile schema
- Clear error messages for version mismatches
- Migration tools if format changes significantly

## Future Enhancements

1. **CSS/Script template coverage** — track coverage in style and script blocks
2. **IDE integration** — LSP extension to show coverage in editor
3. **Differential coverage** — show coverage changes between commits
4. **Component-level metrics** — which components are never instantiated
5. **Branch coverage detail** — track if/else separately from execution count
6. **Coverage badges** — generate SVG badges for README

## Decision Log

- **Use filenames instead of hashes:** Prioritizes human/AI readability over minor disk space savings
- **Track elements and text:** More coverage data aids debugging, noise can be filtered
- **Skip CSS/script in v1:** Different AST structure deserves separate design
- **JSON profile format:** Human-readable, AI-parseable, widely supported
- **Directory-based workflow:** Mirrors GOCOVERDIR, familiar to Go developers
- **Terminal default, HTML opt-in:** Simpler interface, predictable behavior
- **Parser Ranges over SourceMap:** SourceMap is incomplete by design, Ranges already used for errors
