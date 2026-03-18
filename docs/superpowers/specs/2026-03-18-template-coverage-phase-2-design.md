# Template Coverage Phase 2 - Complete Coverage Points

## Goal

Extend template coverage instrumentation to track all expression and element types, achieving comprehensive coverage across template calls, control flow (if/else, switch, for), HTML elements, text literals, and children expressions.

## Background

Phase 1 implemented the core coverage infrastructure:
- Runtime registry for tracking hits (`runtime/coverage.go`)
- JSON profile format
- Merge tooling for combining profiles
- Generator support via `--coverage` flag
- Instrumentation for string expressions only

Phase 2 completes the coverage system by instrumenting the remaining trackable points.

## Architecture

### Coverage Tracking Model

The architecture remains unchanged from Phase 1:

1. **Generation time:** Parser provides source positions via `Range` fields on AST nodes
2. **Code emission:** Generator emits `templruntime.CoverageTrack(filename, line, col)` calls before each trackable point
3. **Runtime:** Coverage registry records hits when `TEMPLCOVERDIR` is set
4. **Test cleanup:** Tests call `FlushCoverage()` to write JSON profiles
5. **Analysis:** Merge tool combines profiles from multiple test runs

### Coverage Points Tracked in Phase 2

Phase 2 adds instrumentation for:

1. **If/else branches** - Track condition, then-branch entry, else-branch entry
2. **Switch cases** - Track switch expression, each case entry, default case
3. **For loops** - Track for statement, loop body entry
4. **Template calls** - Track call site when invoking another template
5. **Static HTML elements** - Track opening tag, closing tag (separate points)
6. **Text literals** - Track static text nodes
7. **Children expressions** - Track children block rendering

### Why These Granularities?

**If/else:** Tracking each branch separately provides true branch coverage, catching untested else clauses.

**Switch:** Tracking each case helps identify untested switch paths, similar to branch coverage.

**For loops:** Tracking statement + body distinguishes "loop never ran" from "loop ran N times", catching empty collection bugs.

**Elements:** Tracking opening and closing tags separately helps identify incomplete renders in streaming scenarios.

**Template calls:** Tracking call sites shows which component invocations executed, while called templates have their own internal coverage.

**Text/children:** Provides complete picture of which template sections executed, even static parts.

## Implementation Approach

### Pattern: Individual Method Instrumentation

Following Phase 1's pattern in `writeStringExpression`, each of the 7 write methods will add coverage tracking:

```go
if g.options.Coverage {
    line := node.Range.From.Line
    col := node.Range.From.Col
    filename := g.options.FileName
    trackingCall := fmt.Sprintf("templruntime.CoverageTrack(%q, %d, %d)\n",
        filename, line, col)
    if _, err = g.w.WriteIndent(indentLevel, trackingCall); err != nil {
        return err
    }
}
```

### Methods to Instrument

1. **`writeIfExpression`** (generator.go ~line 728)
   - Add tracking before if condition
   - Add tracking at then-branch entry
   - Add tracking at else-branch entry (if present)

2. **`writeSwitchExpression`** (generator.go ~line 792)
   - Add tracking before switch expression
   - Add tracking at each case entry
   - Add tracking at default case entry (if present)

3. **`writeForExpression`** (generator.go ~line 932)
   - Add tracking before for statement
   - Add tracking at loop body entry

4. **`writeTemplElementExpression`** (generator.go ~line 840)
   - Add tracking before Render() call

5. **`writeElement`** (generator.go ~line 998)
   - Add tracking before opening tag write
   - Add tracking before closing tag write (skip void elements)

6. **`writeText`** (generator.go ~line 1720)
   - Add tracking before text write

7. **`writeChildrenExpression`** (generator.go ~line 830)
   - Add tracking before children render

### Why Individual Instrumentation?

**Alternatives considered:**
- Extract helper functions for DRY
- Add coverage metadata layer

**Chosen approach:** Direct instrumentation in each method

**Rationale:**
- Follows working Phase 1 pattern
- Each method is self-contained and easy to understand
- Minimal code duplication (just the if-check and CoverageTrack call)
- No architectural changes needed
- Easier to debug and reason about
- Coverage logic is visible where it executes

## Error Handling

**Coverage is non-blocking:**
- Only emits tracking when `g.options.Coverage` is true
- Zero runtime overhead when coverage disabled
- Tracking errors should not fail rendering

**Error scenarios:**
- **Missing Range data:** Skip tracking if `Range.From` is invalid (line/col == 0)
- **Coverage disabled:** Early return from tracking code
- **Emission errors:** Propagate up like other generation errors

**Consistency with Phase 1:**
- No special error handling beyond if-check
- Runtime `CoverageTrack()` already handles nil registry
- Same pattern across all instrumented methods

## Testing Strategy

### Test Structure

Extend `generator/test-coverage/` with new test templates exercising each coverage point type.

### Coverage Points to Test

1. **If/else branches:**
   - Template with if-then (condition true)
   - Template with if-then-else (both paths)
   - Verify: condition tracked, then-branch tracked, else-branch tracked

2. **Switch cases:**
   - Template with switch + multiple cases
   - Execute different cases
   - Verify: switch tracked, each case tracked

3. **For loops:**
   - Template with non-empty collection
   - Template with empty collection
   - Verify: for statement tracked, body tracked when non-empty

4. **Template calls:**
   - Template calling another template
   - Verify: call site tracked

5. **Static HTML elements:**
   - Template with regular elements (div, button)
   - Template with void elements (input, img)
   - Verify: opening tags tracked, closing tags tracked (except void)

6. **Text literals:**
   - Template with static text
   - Verify: text tracked

7. **Children expressions:**
   - Template rendering children
   - Verify: children expression tracked

### Acceptance Criteria

- Each coverage point type has at least one test
- Coverage profiles show expected hits
- No regression in Phase 1 (string expressions still work)
- Generated code compiles and runs
- Tests pass with and without `--coverage` flag

## Non-Goals

**Not in Phase 2:**
- CSS template coverage (documented limitation)
- Script template coverage (documented limitation)
- Coverage visualization/HTML reports (separate feature)
- Performance optimization of tracking overhead
- Alternative profile formats

## Implementation Plan

Phase 2 will be implemented following the established pattern:

1. Instrument `writeIfExpression` (if/else branches)
2. Instrument `writeSwitchExpression` (switch cases)
3. Instrument `writeForExpression` (for loops)
4. Instrument `writeTemplElementExpression` (template calls)
5. Instrument `writeElement` (HTML elements)
6. Instrument `writeText` (text literals)
7. Instrument `writeChildrenExpression` (children expressions)

Each step adds instrumentation, updates tests, and verifies coverage profiles.

## Success Metrics

- All 7 write methods instrumented
- Test coverage demonstrates each point type works
- No regressions in existing coverage functionality
- Documentation updated to reflect Phase 2 completion
