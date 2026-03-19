# Template Coverage Phase 3: Reporting — Design Spec

## Goal

Add a `templ coverage report` command that generates coverage reports in three formats (terminal, HTML, JSON), and extend `templ generate --coverage` to emit a coverage manifest listing all possible coverage points. Together these enable developers to see coverage percentages, identify untested template code, and integrate coverage data into CI pipelines.

## Scope

**In scope:**
- Coverage manifest generation during `templ generate --coverage`
- `templ coverage report` command with terminal, HTML, and JSON output
- Coverage percentage calculation (covered points / total points)

**Out of scope:**
- Threshold checking / CI enforcement (Phase 4)
- Per-type coverage breakdown (future enhancement)
- Runtime type tracking changes
- CSS and script template coverage

## Architecture

Phase 3 adds two components to the existing coverage system:

1. **Coverage manifest** — `templ generate --coverage` emits a manifest file listing every coverage point (file, line, col). This is the denominator for percentage calculations.

2. **Report command** — `templ coverage report` reads a coverage profile (hits) and manifest (total points), then outputs a report in one of three formats.

No runtime changes are required. The existing `CoverageTrack(filename, line, col)` signature stays as-is.

### Data Flow

```
templ generate --coverage
    ├── template_templ.go  (instrumented code, existing)
    └── coverage-manifest.json  (NEW: all possible coverage points)

go test ./...  (with TEMPLCOVERDIR set)
    └── coverage/templ-*.json  (coverage profiles, existing)

templ coverage report -i=coverage/*.json -m=coverage-manifest.json
    ├── terminal output (default)
    ├── --html → coverage.html
    └── --json → JSON to stdout or file
```

### Definitions

A coverage point is considered **covered** if it appears in the profile with `hits > 0`. Points are matched between manifest and profile by exact `(filename, line, col)` tuple.

## Coverage Manifest

### Generation

During `templ generate --coverage`, the generator already walks the AST and emits `CoverageTrack` calls. At each instrumentation site, it also collects the coverage point (file, line, col).

**Collection mechanism:** The `generator.Generate()` function processes one file at a time and returns a `GeneratorOutput` struct. Extend `GeneratorOutput` to include a `CoveragePoints []ManifestPoint` field. Each coverage instrumentation site appends to this slice during generation. After all files are generated (the walk completes), the `Run()` function in `generatecmd` aggregates points from all outputs and writes the manifest.

**Concurrency:** File generation can run concurrently. Each generator instance collects points into its own slice (no sharing). Aggregation happens after all files complete, so no synchronization is needed during generation.

**Watch mode:** Not supported for manifest generation. The manifest is only written during a full `templ generate --coverage` run. Watch mode (`--watch`) skips manifest output. Developers should re-run `templ generate --coverage` to refresh the manifest after changes.

### CLI Interface (templ generate)

```
templ generate --coverage [--coverage-manifest=path]

Flags:
  --coverage            Enable coverage instrumentation (existing)
  --coverage-manifest   Output path for coverage manifest.
                        Default: coverage-manifest.json in working directory.
```

### Format

```json
{
  "version": "1",
  "files": {
    "templates/user/profile.templ": [
      {"line": 5, "col": 3},
      {"line": 8, "col": 2},
      {"line": 12, "col": 1}
    ]
  }
}
```

### Location

Written to `coverage-manifest.json` in the working directory by default. Configurable via `--coverage-manifest=path` flag on `templ generate`.

### Properties

- Static artifact of generation, not runtime. Changes only when templates change and are regenerated.
- Small file — just position tuples, no source content or hit data.
- Deterministic ordering: files sorted alphabetically, points within each file sorted by `(line, col)` ascending.

## Report Command

### CLI Interface

```
templ coverage report [flags]

Flags:
  -i      Input coverage profile(s). Supports comma-separated paths and glob
          patterns (same format as templ coverage merge). Multiple files are
          auto-merged. Required.
  -m      Coverage manifest file. Required for percentages; without it, only
          hit counts are shown with a warning.
  --html  Generate HTML report instead of terminal output.
  --json  Generate JSON report instead of terminal output.
  -o      Output file. Defaults: stdout (terminal/JSON), coverage.html (HTML).
```

### Terminal Output (Default)

```
templates/user/profile.templ      85.7%  (6/7)
templates/user/settings.templ    100.0%  (4/4)
templates/layout/base.templ       50.0%  (3/6)
total                             76.5%  (13/17)
```

- Per-file coverage percentage with covered/total counts
- Sorted alphabetically by file path
- Total line at the bottom
- Files in manifest but absent from profile shown as 0.0%

### HTML Output

Single self-contained HTML file with inlined CSS. No JavaScript required for basic functionality.

**Layout:**
- File selector dropdown at top (lists all template files)
- Source code view with line numbers
- Coverage summary bar showing overall percentage

**Source annotation:**
- Reads each `.templ` file from disk using paths in the manifest
- Lines with covered points: green background
- Lines with uncovered points: red background
- Lines with no coverage points: no highlight (neutral)
- Lines with both covered and uncovered points: yellow/amber (partial)

For HTML annotation, all coverage points sharing the same line number are aggregated. A line is green if all its points are covered, red if none are, and yellow if partially covered. Coverage applies only to the line of each point's `(line, col)` position, not to a range of lines.

**File selector:** Minimal JavaScript for switching between files. All file contents inlined in the HTML as hidden divs; JS toggles visibility.

**Source not found:** If a `.templ` file can't be read from disk, show "Source not available" for that file. The file still appears in the summary with its percentage.

**Default output:** `coverage.html` if `-o` is not specified.

### JSON Output

```json
{
  "version": "1",
  "total": {
    "covered": 13,
    "total": 17,
    "percentage": 76.5
  },
  "files": {
    "templates/user/profile.templ": {
      "covered": 6,
      "total": 7,
      "percentage": 85.7
    },
    "templates/layout/base.templ": {
      "covered": 3,
      "total": 6,
      "percentage": 50.0
    }
  }
}
```

Aggregate summary distinct from the raw coverage profile. Designed for CI scripts, dashboards, or AI agents.

When `-m` is not provided, `percentage` and `total` fields are omitted from JSON output. Only `covered` (number of points with hits > 0) is included per file.

Output to stdout by default, or to a file with `-o`.

### Auto-Merge

If `-i` matches multiple files via glob, merge them automatically using the same logic as `templ coverage merge`. This avoids requiring a separate merge step before reporting.

## Error Handling

- **Missing `-i` flag:** Error with usage hint.
- **Missing `-m` flag:** Report works but shows hit counts only, no percentages. Prints warning: `"No manifest provided (-m); coverage percentages unavailable."`
- **No matching profile files:** Error: `"No coverage profiles found matching <pattern>."`
- **Source file not found (HTML):** Per-file graceful degradation — show "Source not available" for that file, continue with others.
- **Manifest lists files not in profile:** Show as 0.0% coverage (untested templates).
- **Profile has files not in manifest:** Include with hit counts but no percentage (stale manifest).
- **Invalid JSON in profile or manifest:** Error with filename and parse error details.

## Testing Strategy

- **Manifest generation:** Test that `templ generate --coverage` produces a correct manifest with expected points for a known template.
- **Terminal report:** Golden-file test — generate a report from a known profile + manifest, compare against expected output string.
- **HTML report:** Test that output contains expected elements (file selector, green/red highlighted lines, coverage percentages). Parse HTML to verify structure rather than exact string matching.
- **JSON report:** Unmarshal output and assert on structure, percentages, and covered/total counts.
- **Edge cases:** Missing manifest (warning, no percentages), missing source files (graceful degradation), empty profile (all 0%), stale manifest (extra files in profile).
- **Integration test:** End-to-end from `templ generate --coverage` through test execution through `templ coverage report` — verify the full pipeline produces correct output.
