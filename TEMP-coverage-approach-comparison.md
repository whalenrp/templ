# Templ Coverage: Approach Comparison

We evaluated three approaches as alternatives to the `CoverageTrack` system on
`coverage-final-3`, motivated by a desire to reduce custom infrastructure
(particularly the process-shutdown flush logic and custom profile format).

## Approach 1: Source map post-processing

Use the templ generator's existing in-memory source map to remap standard
`go test -cover` profiles from `_templ.go` coordinates to `.templ` coordinates.

**Finding: not viable.** The source map only covers expression positions
(variables like `show`, `items`, `item`), not HTML elements or static text. Of
239 source map entries, 149 fell in gaps between coverage blocks and produced no
data. HTML elements — which are just `WriteString` calls in the generated Go and
are fully instrumented by `go test -cover` — have no source map entries, so they
can't be remapped at all. Abandoned early.

Prototype: `cmd/TEMP-coverage-remap/`, `generator/TEMP-test-sourcemap-coverage/`

---

## Approach 2: `//line` directives

Replace each `CoverageTrack(...)` call in the generator with a
`//line template.templ:N` comment directive at the same position. The Go
coverage tool reads these directives and uses their line numbers in the coverage
profile, so the profile carries templ coordinates under the `_templ.go`
filename. Remapping becomes a trivial filename substitution.

### What works

- Line numbers in the profile are redirected to templ coordinates — confirmed
  with a prototype test
- Branch coverage attribution is correct (true branch covered, else not)
- Eliminates the custom runtime entirely (`CoverageTrack`, `CoverageRegistry`,
  `RunWithCoverage`, the profile flush-on-shutdown concern)
- No changes to how users run tests

### What doesn't work — backwards coverage blocks

The Go coverage tool produces blocks where `EndLine < StartLine` ("backwards
blocks") in two structural cases that cannot be avoided:

**1. Boilerplate transition.** Every generated function has ~20 lines of
boilerplate setup before the first template construct. The first
`//line template.templ:4` directive resets the virtual counter from ~31 back to
4. The coverage block spanning the boilerplate ends at virtual line 4, producing
e.g. `template_templ.go:31.3,4.0` — StartLine 31, EndLine 4.

**2. Else branches.** `} else {` cannot be separated from `}` on the preceding
line, so a `//line` directive cannot be placed before it. The else entry block
opens at the natural virtual position (e.g. line 9), then a `//line` inside the
else body resets to a lower templ line (e.g. 7), causing the block to close at
a line lower than it opened — e.g. `9.0,8.0`.

Example (calling `Greet(true)` only):

```go
// template_templ.go (error handling omitted)
// [vN] = virtual line number seen by the coverage tool

                                       // [v31] ctx = templ.ClearChildren(ctx)
//line template.templ:4                // resets counter: v31 → v4
    if show {                          // [v4]  ← BLOCK 1 opens  covered ✓
//line template.templ:5                // counter continues forward: v5
        WriteString("<p>Hello!</p>")   // [v5]
        if err != nil { return err }   // [v6]  ← BLOCK 2 opens  not hit ✗
    }                                  // [v8]
    } else {                           // [v9]  ← BLOCK 3 opens  not covered ✗
//line template.templ:7                // resets counter: v9 → v7  (BACKWARDS)
        WriteString("<p>Goodbye!</p>") // [v7]
        if err != nil { return err }   // [v8]  ← BLOCK 4 opens
    }
```

Resulting profile:

```
template_templ.go:4.0,6.0   count=1  ← BLOCK 1  covered ✓   (forward, ok)
template_templ.go:6.0,8.0   count=0  ← BLOCK 2  no error ✗  (forward, ok)
template_templ.go:9.0,8.0   count=0  ← BLOCK 3  else ✗      (BACKWARDS: EndLine 8 < StartLine 9)
template_templ.go:31.3,4.0  count=1  ← boilerplate ✓        (BACKWARDS: EndLine 4 < StartLine 31)
```

**This is fundamental Go behavior, not fixable in post-processing.** The profile
is written by `cmd/cover` / the Go compiler before any downstream tooling
touches it. Standard tools — `go tool cover`, `go tool covdata`, IDE coverage
overlays — all expect monotonically increasing line ranges and will either reject
or misrender backwards blocks. Any system consuming `//line`-produced profiles
needs special handling for backwards blocks, meaning the generator change is only
useful paired with custom code. That fails the open-source compatibility
requirement.

### Prototype

`generator/TEMP-test-linedirective/` — the test currently fails on Assertion 3
(backwards block detection). Reproduce with:

```bash
go test -v -run '^TestTEMP_LineDirecCoverage$' ./generator/TEMP-test-linedirective/
```

---

## Approach 3: `CoverageTrack` (current `coverage-final-3`)

The generator emits a `CoverageTrack("file.templ", line, col)` call before each
template construct. A small runtime package records hits in a thread-safe
registry during test execution and flushes to `TEMPLCOVERDIR` on process exit.
Separate `templ coverage merge` and `templ coverage report` commands aggregate
and display results.

### What works

- Accurate coverage for all construct types: HTML elements, control flow, static
  text, template calls, children expressions
- No backwards blocks — profiles are valid, well-formed files
- No dependency on Uber-specific infrastructure; works identically anywhere
- Correct construct-level attribution, not just branch-level

### Cost

- Custom runtime (~200 lines: `CoverageTrack`, `CoverageRegistry`,
  `RunWithCoverage`)
- Custom JSON profile format (not directly compatible with `go tool cover`)
- Custom `templ coverage merge` command
- Tests need a `TestMain` wrapper calling `RunWithCoverage(m)` to ensure the
  flush happens before process exit
- `TEMPLCOVERDIR` env var that users need to set

---

## On integrating with Uber's coverage infrastructure

The suggestion of plugging into Uber's baseline-from-AST + merge pipeline is
architecturally sound, but still requires a mechanism to map `_templ.go`
coverage blocks back to `.templ` positions. `//line` directives were the
proposed mapping mechanism, but the backwards blocks issue is not fixable at the
Uber tooling layer — it's produced by the Go toolchain before any Uber code runs.
The most viable path would be bridging `CoverageTrack` output into Uber's merge
pipeline (a thin translation layer from the custom JSON format) rather than
replacing the generator instrumentation.

---

## Summary

| | Source map | `//line` directives | `CoverageTrack` |
|---|---|---|---|
| HTML element coverage | ✗ no entries | ✓ as WriteString | ✓ explicit |
| Branch coverage accuracy | partial | ✓ | ✓ |
| Backwards blocks | n/a | ✗ structural, unfixable | ✓ none |
| Standard Go tool compatible | ✓ | ✗ | partial (custom format) |
| Generator changes needed | none | yes | yes |
| Custom runtime needed | none | none | yes (~200 lines) |
| Open-source compatible | ✓ (partial) | ✗ | ✓ |
| Works outside Uber | ✓ (partial) | ✗ | ✓ |
