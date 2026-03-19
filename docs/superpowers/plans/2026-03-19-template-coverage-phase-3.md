# Template Coverage Phase 3: Reporting — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add coverage manifest generation and a `templ coverage report` command with terminal, HTML, and JSON output formats.

**Architecture:** Extend `GeneratorOutput` to collect coverage points during generation, aggregate them into a manifest file after all files are processed, then add a `report` subcommand under `templ coverage` that reads the manifest + profile to produce reports. The report command reuses the existing profile loading and merging infrastructure.

**Tech Stack:** Go, templ generator, `html/template` for HTML report generation

---

## File Structure

### Files to Create

**`cmd/templ/coveragecmd/manifest.go`** — Manifest data types and I/O
- `ManifestPoint` struct (line, col)
- `Manifest` struct (version, files map)
- `LoadManifest(path)` and `(m *Manifest) Write(path)` functions

**`cmd/templ/coveragecmd/report.go`** — Report generation logic
- `runReport()` function (CLI flag parsing and dispatch)
- `expandInputPaths()` helper (extracted from `runMerge`)
- `generateTerminalReport()`, `generateHTMLReport()`, `generateJSONReport()`

**`cmd/templ/coveragecmd/report_test.go`** — Report tests

**`cmd/templ/coveragecmd/manifest_test.go`** — Manifest I/O tests

### Files to Modify

**`generator/generator.go`** — Add `CoveragePoints` field to `GeneratorOutput`, collect points at each instrumentation site

**`cmd/templ/coveragecmd/main.go`** — Add `report` subcommand routing, extract `expandInputPaths` for reuse

**`cmd/templ/generatecmd/cmd.go`** — Add `CoverageManifest` flag to Arguments, write manifest after full generation

**`cmd/templ/generatecmd/main.go`** — Add `--coverage-manifest` flag parsing

**`cmd/templ/generatecmd/eventhandler.go`** — Collect coverage points from GeneratorOutput into manifest

**`docs/docs/09-developer-tools/07-coverage.md`** — Document the report command

---

## Task 1: Manifest Data Types and I/O

**Files:**
- Create: `cmd/templ/coveragecmd/manifest.go`
- Create: `cmd/templ/coveragecmd/manifest_test.go`

- [ ] **Step 1: Write failing test for manifest round-trip**

Create `cmd/templ/coveragecmd/manifest_test.go`:
```go
package coveragecmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	m := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			"templates/a.templ": {
				{Line: 5, Col: 3},
				{Line: 8, Col: 2},
			},
			"templates/b.templ": {
				{Line: 1, Col: 0},
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	if err := m.Write(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Version != "1" {
		t.Errorf("version: got %q, want %q", loaded.Version, "1")
	}
	if len(loaded.Files["templates/a.templ"]) != 2 {
		t.Errorf("a.templ points: got %d, want 2", len(loaded.Files["templates/a.templ"]))
	}
	if len(loaded.Files["templates/b.templ"]) != 1 {
		t.Errorf("b.templ points: got %d, want 1", len(loaded.Files["templates/b.templ"]))
	}
}

func TestLoadManifestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/templ/coveragecmd/ -run TestManifest -v
```

Expected: FAIL — `ManifestPoint`, `Manifest`, `LoadManifest` undefined

- [ ] **Step 3: Write manifest implementation**

Create `cmd/templ/coveragecmd/manifest.go`:
```go
package coveragecmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ManifestPoint represents a coverage point location in a template file.
type ManifestPoint struct {
	Line uint32 `json:"line"`
	Col  uint32 `json:"col"`
}

// Manifest lists all possible coverage points, used as the denominator for percentage calculations.
type Manifest struct {
	Version string                      `json:"version"`
	Files   map[string][]ManifestPoint  `json:"files"`
}

// LoadManifest reads a coverage manifest from a JSON file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return &m, nil
}

// Write saves the manifest to a JSON file with deterministic ordering.
func (m *Manifest) Write(path string) error {
	// Sort files alphabetically and points by (line, col) for deterministic output
	sorted := &Manifest{
		Version: m.Version,
		Files:   make(map[string][]ManifestPoint, len(m.Files)),
	}
	for filename, points := range m.Files {
		pts := make([]ManifestPoint, len(points))
		copy(pts, points)
		sort.Slice(pts, func(i, j int) bool {
			if pts[i].Line != pts[j].Line {
				return pts[i].Line < pts[j].Line
			}
			return pts[i].Col < pts[j].Col
		})
		sorted.Files[filename] = pts
	}

	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./cmd/templ/coveragecmd/ -run TestManifest -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/templ/coveragecmd/manifest.go cmd/templ/coveragecmd/manifest_test.go
git commit -m "feat(coverage): add manifest data types and I/O"
```

---

## Task 2: Collect Coverage Points in Generator

**Files:**
- Modify: `generator/generator.go:70-74` (GeneratorOutput struct)
- Modify: `generator/generator.go` (each coverage instrumentation site)

- [ ] **Step 1: Add CoveragePoints field to GeneratorOutput**

In `generator/generator.go`, modify the `GeneratorOutput` struct (line 70):
```go
type GeneratorOutput struct {
	Options        GeneratorOptions     `json:"meta"`
	SourceMap      *parser.SourceMap    `json:"sourceMap"`
	Literals       []string             `json:"literals"`
	CoveragePoints []CoveragePoint      `json:"coveragePoints,omitempty"`
}

// CoveragePoint represents a coverage instrumentation location in the template source.
type CoveragePoint struct {
	Line uint32
	Col  uint32
}
```

- [ ] **Step 2: Collect points at each instrumentation site**

At each existing `if g.options.Coverage {` block, after the `fmt.Sprintf` and `WriteIndent` call, append the point to `g.output.CoveragePoints`. The generator struct has an `output` field — find it and add the append.

First, check if the generator struct stores output. Look at the `generate` method. The `Generate()` function (line 136) creates a `generator` struct and calls methods on it. The generator has a `sourceMap` field but we need to check if it also has the output struct.

Actually, looking at `Generate()` more closely, it builds the `GeneratorOutput` at the end from parts. We need a collector on the generator struct.

Add a field to the `generator` struct:
```go
coveragePoints []CoveragePoint
```

Then at each coverage instrumentation site, after the existing `WriteIndent` call, add:
```go
g.coveragePoints = append(g.coveragePoints, CoveragePoint{Line: n.Range.From.Line, Col: n.Range.From.Col})
```

Finally, in `Generate()` where `GeneratorOutput` is assembled, include:
```go
op.CoveragePoints = g.coveragePoints
```

There are approximately 10 instrumentation sites to update. Each one already has the line/col available in the `fmt.Sprintf` call.

- [ ] **Step 3: Write test for coverage point collection**

Add a test in the existing `generator/test-coverage-if/render_test.go` or create a small test that generates a template and checks that `GeneratorOutput.CoveragePoints` is populated. However, since the existing tests verify coverage at runtime (via profiles), a simpler approach is to test this via the manifest integration test in Task 4. For now, verify the code compiles:

```bash
go build ./generator/
```

Expected: Success

- [ ] **Step 4: Commit**

```bash
git add generator/generator.go
git commit -m "feat(coverage): collect coverage points in GeneratorOutput"
```

---

## Task 3: Generate Manifest During templ generate

**Files:**
- Modify: `cmd/templ/generatecmd/main.go:155-177` (Arguments struct)
- Modify: `cmd/templ/generatecmd/cmd.go:55-170` (Run function)
- Modify: `cmd/templ/generatecmd/eventhandler.go` (collect points from GeneratorOutput)

- [ ] **Step 1: Add CoverageManifest flag to Arguments**

In `cmd/templ/generatecmd/main.go`, add to the `Arguments` struct (after line 172):
```go
CoverageManifest string
```

In `NewArguments()`, find where `--coverage` is parsed (line 94) and add after it:
```go
cmd.StringVar(&cmdArgs.CoverageManifest, "coverage-manifest", "coverage-manifest.json", "Output path for coverage manifest (used with --coverage)")
```

- [ ] **Step 2: Add manifest writing to Run()**

In `cmd/templ/generatecmd/cmd.go`, the `Run()` function needs to:
1. Pass coverage manifest path into the event handler
2. After generation completes (after `grp.Wait()` at line 161), write the manifest

Add a `coveragePoints` map to `FSEventHandler` to accumulate points across files. In the `generate()` method of `eventhandler.go`, after `generator.Generate()` returns, copy `generatorOutput.CoveragePoints` into the handler's map keyed by filename.

In `cmd.go` after `grp.Wait()` (line 163), if `cmd.Args.Coverage && cmd.Args.CoverageManifest != "" && !cmd.Args.Watch`:
```go
if cmd.Args.Coverage && cmd.Args.CoverageManifest != "" && !cmd.Args.Watch {
	manifest := fseh.BuildManifest()
	if err := manifest.Write(cmd.Args.CoverageManifest); err != nil {
		return fmt.Errorf("failed to write coverage manifest: %w", err)
	}
	cmd.Log.Info("Coverage manifest written", slog.String("path", cmd.Args.CoverageManifest))
}
```

- [ ] **Step 3: Add coverage point collection to FSEventHandler**

In `cmd/templ/generatecmd/eventhandler.go`, add to `FSEventHandler`:
```go
coveragePointsMu sync.Mutex
coveragePoints   map[string][]generator.CoveragePoint
```

Initialize it in `NewFSEventHandler()`:
```go
coveragePoints: make(map[string][]generator.CoveragePoint),
```

In `generate()` (around line 235 where `generatorOutput` is used), add:
```go
if len(generatorOutput.CoveragePoints) > 0 {
	h.coveragePointsMu.Lock()
	h.coveragePoints[fileName] = generatorOutput.CoveragePoints
	h.coveragePointsMu.Unlock()
}
```

Add a `BuildManifest()` method. The import of `coveragecmd` from `generatecmd` is safe — it's a unidirectional dependency (no cycle).
```go
func (h *FSEventHandler) BuildManifest() *coveragecmd.Manifest {
	h.coveragePointsMu.Lock()
	defer h.coveragePointsMu.Unlock()
	m := &coveragecmd.Manifest{
		Version: "1",
		Files:   make(map[string][]coveragecmd.ManifestPoint),
	}
	for filename, points := range h.coveragePoints {
		mps := make([]coveragecmd.ManifestPoint, len(points))
		for i, p := range points {
			mps[i] = coveragecmd.ManifestPoint{Line: p.Line, Col: p.Col}
		}
		m.Files[filename] = mps
	}
	return m
}
```

- [ ] **Step 4: Handle single-file mode**

For single-file generation (`-f` flag, line 102-108 of cmd.go), the manifest should also be written. After the `HandleEvent` call:
```go
if cmd.Args.FileName != "" {
	_, err = fseh.HandleEvent(ctx, fsnotify.Event{
		Name: cmd.Args.FileName,
		Op:   fsnotify.Create,
	})
	if err != nil {
		return err
	}
	if cmd.Args.Coverage && cmd.Args.CoverageManifest != "" {
		manifest := fseh.BuildManifest()
		if err := manifest.Write(cmd.Args.CoverageManifest); err != nil {
			return fmt.Errorf("failed to write coverage manifest: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Test manifest generation end-to-end**

```bash
# Generate a test template with coverage + manifest
go run ./cmd/templ generate --coverage --coverage-manifest=/tmp/test-manifest.json -f generator/test-coverage-if/template.templ

# Verify manifest exists and has correct content
cat /tmp/test-manifest.json
```

Expected: JSON file with version "1" and coverage points for `generator/test-coverage-if/template.templ`

- [ ] **Step 6: Update generateUsageText**

In `cmd/templ/generatecmd/main.go`, add to the `generateUsageText` constant (after the `-coverage` entry around line 36):
```
  -coverage-manifest string
    Output path for coverage manifest (used with --coverage). Default: coverage-manifest.json.
```

- [ ] **Step 7: Commit**

```bash
git add cmd/templ/generatecmd/main.go cmd/templ/generatecmd/cmd.go cmd/templ/generatecmd/eventhandler.go
git commit -m "feat(coverage): generate coverage manifest during templ generate"
```

---

## Task 4: Extract expandInputPaths and Add Report Routing

**Files:**
- Modify: `cmd/templ/coveragecmd/main.go`
- Create: `cmd/templ/coveragecmd/report.go`

- [ ] **Step 1: Extract expandInputPaths from runMerge**

In `cmd/templ/coveragecmd/main.go`, extract the glob/comma expansion logic (lines 38-46) into a shared function:
```go
func expandInputPaths(inputPaths string) ([]string, error) {
	var files []string
	for _, pattern := range strings.Split(inputPaths, ",") {
		pattern = strings.TrimSpace(pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}
	return files, nil
}
```

Update `runMerge` to call `expandInputPaths(*inputPaths)`.

- [ ] **Step 2: Add report routing**

In `cmd/templ/coveragecmd/main.go`, update the `Run()` switch (line 16):
```go
case "report":
	return runReport(w, args[1:])
```

Update the usage message (line 13):
```go
return fmt.Errorf("usage: templ coverage <command>\nCommands: merge, report")
```

- [ ] **Step 3: Create report.go with stub**

Create `cmd/templ/coveragecmd/report.go`:
```go
package coveragecmd

import (
	"flag"
	"fmt"
	"io"
)

func runReport(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	inputPaths := fs.String("i", "", "Comma-separated input coverage profiles or glob patterns")
	manifestPath := fs.String("m", "", "Coverage manifest file")
	htmlOutput := fs.Bool("html", false, "Generate HTML report")
	jsonOutput := fs.Bool("json", false, "Generate JSON report")
	outputPath := fs.String("o", "", "Output file path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *inputPaths == "" {
		return fmt.Errorf("-i flag required: specify input coverage profiles")
	}

	// Load and merge profiles
	files, err := expandInputPaths(*inputPaths)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no coverage profiles found matching: %s", *inputPaths)
	}

	profiles := make([]*Profile, 0, len(files))
	for _, file := range files {
		profile, err := LoadProfile(file)
		if err != nil {
			fmt.Fprintf(w, "Warning: skipping %s: %v\n", file, err)
			continue
		}
		profiles = append(profiles, profile)
	}
	if len(profiles) == 0 {
		return fmt.Errorf("no valid profiles loaded")
	}
	merged := MergeProfiles(profiles)

	// Load manifest (optional)
	var manifest *Manifest
	if *manifestPath != "" {
		manifest, err = LoadManifest(*manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}
	} else {
		fmt.Fprintln(w, "Warning: No manifest provided (-m); coverage percentages unavailable.")
	}

	// Dispatch to format-specific generator
	switch {
	case *htmlOutput:
		return generateHTMLReport(w, merged, manifest, *outputPath)
	case *jsonOutput:
		return generateJSONReport(w, merged, manifest, *outputPath)
	default:
		return generateTerminalReport(w, merged, manifest)
	}
}

func generateTerminalReport(w io.Writer, profile *Profile, manifest *Manifest) error {
	return fmt.Errorf("terminal report not yet implemented")
}

func generateHTMLReport(w io.Writer, profile *Profile, manifest *Manifest, outputPath string) error {
	return fmt.Errorf("HTML report not yet implemented")
}

func generateJSONReport(w io.Writer, profile *Profile, manifest *Manifest, outputPath string) error {
	return fmt.Errorf("JSON report not yet implemented")
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./cmd/templ/...
```

Expected: Success

- [ ] **Step 5: Commit**

```bash
git add cmd/templ/coveragecmd/main.go cmd/templ/coveragecmd/report.go
git commit -m "feat(coverage): add report command routing and input loading"
```

---

## Task 5: Terminal Report

**Files:**
- Modify: `cmd/templ/coveragecmd/report.go`
- Create: `cmd/templ/coveragecmd/report_test.go`

- [ ] **Step 1: Write failing test for terminal report**

Create `cmd/templ/coveragecmd/report_test.go`:
```go
package coveragecmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalReport(t *testing.T) {
	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			"a.templ": {
				{Line: 1, Col: 0, Hits: 3},
				{Line: 2, Col: 0, Hits: 0},
			},
			"b.templ": {
				{Line: 1, Col: 0, Hits: 1},
			},
		},
	}
	manifest := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			"a.templ": {{Line: 1, Col: 0}, {Line: 2, Col: 0}},
			"b.templ": {{Line: 1, Col: 0}},
			"c.templ": {{Line: 1, Col: 0}}, // not in profile — should show 0%
		},
	}

	var buf bytes.Buffer
	if err := generateTerminalReport(&buf, profile, manifest); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// a.templ: 1 of 2 covered = 50%
	if !strings.Contains(output, "a.templ") || !strings.Contains(output, "50.0%") {
		t.Errorf("expected a.templ at 50%%, got:\n%s", output)
	}
	// b.templ: 1 of 1 covered = 100%
	if !strings.Contains(output, "b.templ") || !strings.Contains(output, "100.0%") {
		t.Errorf("expected b.templ at 100%%, got:\n%s", output)
	}
	// c.templ: 0 of 1 covered = 0%
	if !strings.Contains(output, "c.templ") || !strings.Contains(output, "0.0%") {
		t.Errorf("expected c.templ at 0%%, got:\n%s", output)
	}
	// total line
	if !strings.Contains(output, "total") {
		t.Errorf("expected total line, got:\n%s", output)
	}
}

func TestTerminalReportWithoutManifest(t *testing.T) {
	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			"a.templ": {{Line: 1, Col: 0, Hits: 3}},
		},
	}

	var buf bytes.Buffer
	if err := generateTerminalReport(&buf, profile, nil); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// Should show file with hit count but no percentage
	if !strings.Contains(output, "a.templ") {
		t.Errorf("expected a.templ in output, got:\n%s", output)
	}
	if strings.Contains(output, "%") {
		t.Errorf("expected no percentages without manifest, got:\n%s", output)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/templ/coveragecmd/ -run TestTerminalReport -v
```

Expected: FAIL — "terminal report not yet implemented"

- [ ] **Step 3: Implement terminal report**

In `cmd/templ/coveragecmd/report.go`, replace the `generateTerminalReport` stub:
```go
func generateTerminalReport(w io.Writer, profile *Profile, manifest *Manifest) error {
	type fileStat struct {
		name    string
		covered int
		total   int
	}

	var stats []fileStat

	if manifest != nil {
		// Use manifest as source of truth for all files
		for filename, mPoints := range manifest.Files {
			covered := 0
			pPoints := profile.Files[filename]
			// Build set of covered positions from profile
			coveredSet := make(map[Position]bool)
			for _, p := range pPoints {
				if p.Hits > 0 {
					coveredSet[Position{Line: p.Line, Col: p.Col}] = true
				}
			}
			for _, mp := range mPoints {
				if coveredSet[Position{Line: mp.Line, Col: mp.Col}] {
					covered++
				}
			}
			stats = append(stats, fileStat{name: filename, covered: covered, total: len(mPoints)})
		}
		// Include profile-only files (stale manifest)
		for filename, pPoints := range profile.Files {
			if _, inManifest := manifest.Files[filename]; !inManifest {
				covered := 0
				for _, p := range pPoints {
					if p.Hits > 0 {
						covered++
					}
				}
				stats = append(stats, fileStat{name: filename, covered: covered, total: -1})
			}
		}
	} else {
		// No manifest — just show hit counts
		for filename, pPoints := range profile.Files {
			covered := 0
			for _, p := range pPoints {
				if p.Hits > 0 {
					covered++
				}
			}
			stats = append(stats, fileStat{name: filename, covered: covered, total: -1})
		}
	}

	// Sort alphabetically
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].name < stats[j].name
	})

	// Find max filename length for alignment
	maxLen := len("total")
	for _, s := range stats {
		if len(s.name) > maxLen {
			maxLen = len(s.name)
		}
	}

	// Print rows
	totalCovered, totalTotal := 0, 0
	hasPercentages := manifest != nil
	for _, s := range stats {
		totalCovered += s.covered
		if s.total >= 0 {
			totalTotal += s.total
			fmt.Fprintf(w, "%-*s  %5.1f%%  (%d/%d)\n", maxLen, s.name,
				percentage(s.covered, s.total), s.covered, s.total)
		} else {
			fmt.Fprintf(w, "%-*s  %d points covered\n", maxLen, s.name, s.covered)
		}
	}

	// Total line
	if hasPercentages {
		fmt.Fprintf(w, "%-*s  %5.1f%%  (%d/%d)\n", maxLen, "total",
			percentage(totalCovered, totalTotal), totalCovered, totalTotal)
	} else {
		fmt.Fprintf(w, "%-*s  %d points covered\n", maxLen, "total", totalCovered)
	}

	return nil
}

func percentage(covered, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
}
```

Add `"sort"` to the imports.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./cmd/templ/coveragecmd/ -run TestTerminalReport -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/templ/coveragecmd/report.go cmd/templ/coveragecmd/report_test.go
git commit -m "feat(coverage): implement terminal report output"
```

---

## Task 6: JSON Report

**Files:**
- Modify: `cmd/templ/coveragecmd/report.go`
- Modify: `cmd/templ/coveragecmd/report_test.go`

- [ ] **Step 1: Write failing test for JSON report**

Add to `cmd/templ/coveragecmd/report_test.go`:
```go
func TestJSONReport(t *testing.T) {
	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			"a.templ": {
				{Line: 1, Col: 0, Hits: 3},
				{Line: 2, Col: 0, Hits: 0},
			},
		},
	}
	manifest := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			"a.templ": {{Line: 1, Col: 0}, {Line: 2, Col: 0}},
		},
	}

	var buf bytes.Buffer
	if err := generateJSONReport(&buf, profile, manifest, ""); err != nil {
		t.Fatal(err)
	}

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}

	if report.Version != "1" {
		t.Errorf("version: got %q, want %q", report.Version, "1")
	}
	if report.Total.Covered != 1 {
		t.Errorf("total covered: got %d, want 1", report.Total.Covered)
	}
	if report.Total.Total != 2 {
		t.Errorf("total total: got %d, want 2", report.Total.Total)
	}
	if report.Total.Percentage != 50.0 {
		t.Errorf("total percentage: got %f, want 50.0", report.Total.Percentage)
	}
	fileStat := report.Files["a.templ"]
	if fileStat.Covered != 1 || fileStat.Total != 2 {
		t.Errorf("a.templ: got covered=%d total=%d, want 1/2", fileStat.Covered, fileStat.Total)
	}
}

func TestJSONReportWithoutManifest(t *testing.T) {
	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			"a.templ": {{Line: 1, Col: 0, Hits: 3}},
		},
	}

	var buf bytes.Buffer
	if err := generateJSONReport(&buf, profile, nil, ""); err != nil {
		t.Fatal(err)
	}

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}

	if report.Total.Total != 0 {
		t.Errorf("expected total=0 without manifest, got %d", report.Total.Total)
	}
	if report.Files["a.templ"].Covered != 1 {
		t.Errorf("expected covered=1, got %d", report.Files["a.templ"].Covered)
	}
}
```

Add `"encoding/json"` to the test imports.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/templ/coveragecmd/ -run TestJSONReport -v
```

Expected: FAIL

- [ ] **Step 3: Implement JSON report**

In `cmd/templ/coveragecmd/report.go`, add types and implementation:
```go
type JSONReport struct {
	Version string                     `json:"version"`
	Total   JSONReportSummary          `json:"total"`
	Files   map[string]JSONReportSummary `json:"files"`
}

type JSONReportSummary struct {
	Covered    int     `json:"covered"`
	Total      int     `json:"total,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
}

func generateJSONReport(w io.Writer, profile *Profile, manifest *Manifest, outputPath string) error {
	report := JSONReport{
		Version: "1",
		Files:   make(map[string]JSONReportSummary),
	}

	allFiles := make(map[string]bool)
	if manifest != nil {
		for f := range manifest.Files {
			allFiles[f] = true
		}
	}
	for f := range profile.Files {
		allFiles[f] = true
	}

	totalCovered, totalTotal := 0, 0
	for filename := range allFiles {
		var covered int
		summary := JSONReportSummary{}

		if manifest != nil {
			if mPoints, ok := manifest.Files[filename]; ok {
				covered = countCoveredAgainstManifest(profile.Files[filename], mPoints)
				summary.Total = len(mPoints)
				summary.Percentage = percentage(covered, len(mPoints))
				totalTotal += len(mPoints)
			} else {
				covered = countCovered(profile.Files[filename])
			}
		} else {
			covered = countCovered(profile.Files[filename])
		}
		summary.Covered = covered
		totalCovered += covered
		report.Files[filename] = summary
	}

	if manifest != nil {
		report.Total = JSONReportSummary{
			Covered:    totalCovered,
			Total:      totalTotal,
			Percentage: percentage(totalCovered, totalTotal),
		}
	} else {
		report.Total = JSONReportSummary{Covered: totalCovered}
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON report: %w", err)
	}

	if outputPath != "" {
		return os.WriteFile(outputPath, data, 0644)
	}
	_, err = w.Write(data)
	return err
}

func countCovered(points []CoveragePoint) int {
	covered := 0
	for _, p := range points {
		if p.Hits > 0 {
			covered++
		}
	}
	return covered
}

// countCoveredAgainstManifest counts manifest points that have a matching profile point with hits > 0.
func countCoveredAgainstManifest(profilePoints []CoveragePoint, manifestPoints []ManifestPoint) int {
	coveredSet := make(map[Position]bool)
	for _, p := range profilePoints {
		if p.Hits > 0 {
			coveredSet[Position{Line: p.Line, Col: p.Col}] = true
		}
	}
	covered := 0
	for _, mp := range manifestPoints {
		if coveredSet[Position{Line: mp.Line, Col: mp.Col}] {
			covered++
		}
	}
	return covered
}
```

Add `"os"` and `"encoding/json"` to imports if not already present.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./cmd/templ/coveragecmd/ -run TestJSONReport -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/templ/coveragecmd/report.go cmd/templ/coveragecmd/report_test.go
git commit -m "feat(coverage): implement JSON report output"
```

---

## Task 7: HTML Report

**Files:**
- Modify: `cmd/templ/coveragecmd/report.go`
- Modify: `cmd/templ/coveragecmd/report_test.go`

- [ ] **Step 1: Write failing test for HTML report**

Add to `cmd/templ/coveragecmd/report_test.go`:
```go
func TestHTMLReport(t *testing.T) {
	// Create a temp .templ file so HTML report can read source
	dir := t.TempDir()
	templFile := filepath.Join(dir, "test.templ")
	os.WriteFile(templFile, []byte("package test\n\ntempl Hello() {\n\t<div>hello</div>\n}\n"), 0644)

	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			templFile: {
				{Line: 3, Col: 1, Hits: 5},
				{Line: 4, Col: 0, Hits: 0},
			},
		},
	}
	manifest := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			templFile: {{Line: 3, Col: 1}, {Line: 4, Col: 0}},
		},
	}

	outputPath := filepath.Join(dir, "coverage.html")
	var buf bytes.Buffer
	if err := generateHTMLReport(&buf, profile, manifest, outputPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)

	if !strings.Contains(html, "<html") {
		t.Error("expected HTML document")
	}
	if !strings.Contains(html, "test.templ") {
		t.Error("expected filename in report")
	}
	if !strings.Contains(html, "covered") {
		t.Error("expected coverage class in report")
	}
}

func TestHTMLReportMissingSource(t *testing.T) {
	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			"/nonexistent/test.templ": {{Line: 1, Col: 0, Hits: 1}},
		},
	}
	manifest := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			"/nonexistent/test.templ": {{Line: 1, Col: 0}},
		},
	}

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "coverage.html")
	var buf bytes.Buffer
	err := generateHTMLReport(&buf, profile, manifest, outputPath)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(outputPath)
	if !strings.Contains(string(data), "Source not available") {
		t.Error("expected 'Source not available' for missing file")
	}
}
```

Add `"os"` and `"path/filepath"` to test imports if not already present.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/templ/coveragecmd/ -run TestHTMLReport -v
```

Expected: FAIL

- [ ] **Step 3: Implement HTML report**

In `cmd/templ/coveragecmd/report.go`, replace the `generateHTMLReport` stub. Use Go's `html/template` package to generate a self-contained HTML file:

```go
func generateHTMLReport(w io.Writer, profile *Profile, manifest *Manifest, outputPath string) error {
	if outputPath == "" {
		outputPath = "coverage.html"
	}

	type lineInfo struct {
		Number int
		Text   string
		Class  string // "covered", "uncovered", "partial", ""
	}
	type fileData struct {
		Name       string
		Lines      []lineInfo
		Covered    int
		Total      int
		Percentage float64
		Available  bool
	}

	// Collect all files
	allFiles := make(map[string]bool)
	if manifest != nil {
		for f := range manifest.Files {
			allFiles[f] = true
		}
	}
	for f := range profile.Files {
		allFiles[f] = true
	}

	var filesData []fileData
	totalCovered, totalTotal := 0, 0

	// Sort filenames
	var filenames []string
	for f := range allFiles {
		filenames = append(filenames, f)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		fd := fileData{Name: filepath.Base(filename)}

		// Compute coverage stats
		var covered int
		if manifest != nil {
			if mp, ok := manifest.Files[filename]; ok {
				covered = countCoveredAgainstManifest(profile.Files[filename], mp)
				fd.Total = len(mp)
				fd.Percentage = percentage(covered, len(mp))
				totalTotal += len(mp)
			} else {
				covered = countCovered(profile.Files[filename])
			}
		} else {
			covered = countCovered(profile.Files[filename])
		}
		fd.Covered = covered
		totalCovered += covered

		// Read source file
		source, err := os.ReadFile(filename)
		if err != nil {
			fd.Available = false
			fd.Lines = []lineInfo{{Number: 1, Text: "Source not available"}}
			filesData = append(filesData, fd)
			continue
		}

		fd.Available = true

		// Build coverage maps for this file
		coveredPositions := make(map[uint32]bool)   // line → has covered point
		uncoveredPositions := make(map[uint32]bool)  // line → has uncovered point

		for _, p := range profile.Files[filename] {
			if p.Hits > 0 {
				coveredPositions[p.Line] = true
			}
		}
		if manifest != nil {
			for _, mp := range manifest.Files[filename] {
				pos := Position{Line: mp.Line, Col: mp.Col}
				isCovered := false
				for _, p := range profile.Files[filename] {
					if p.Line == pos.Line && p.Col == pos.Col && p.Hits > 0 {
						isCovered = true
						break
					}
				}
				if !isCovered {
					uncoveredPositions[mp.Line] = true
				}
			}
		}

		// Build line info
		lines := strings.Split(string(source), "\n")
		for i, line := range lines {
			lineNum := uint32(i) // 0-indexed to match generator output
			li := lineInfo{Number: i + 1, Text: line}
			hasCovered := coveredPositions[lineNum]
			hasUncovered := uncoveredPositions[lineNum]
			if hasCovered && hasUncovered {
				li.Class = "partial"
			} else if hasCovered {
				li.Class = "covered"
			} else if hasUncovered {
				li.Class = "uncovered"
			}
			fd.Lines = append(fd.Lines, li)
		}

		filesData = append(filesData, fd)
	}

	// Render HTML
	tmplData := struct {
		Files           []fileData
		TotalCovered    int
		TotalTotal      int
		TotalPercentage float64
	}{
		Files:           filesData,
		TotalCovered:    totalCovered,
		TotalTotal:      totalTotal,
		TotalPercentage: percentage(totalCovered, totalTotal),
	}

	tmpl, err := htmltemplate.New("coverage").Parse(htmlReportTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return fmt.Errorf("failed to render HTML report: %w", err)
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

const htmlReportTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Templ Coverage Report</title>
<style>
body { font-family: monospace; margin: 0; padding: 20px; background: #1e1e1e; color: #d4d4d4; }
.summary { background: #252526; padding: 15px; margin-bottom: 20px; border-radius: 4px; }
.summary h1 { margin: 0 0 10px; font-size: 18px; }
select { background: #3c3c3c; color: #d4d4d4; border: 1px solid #555; padding: 5px 10px; font-size: 14px; margin-bottom: 15px; }
.file-view { display: none; }
.file-view.active { display: block; }
table { border-collapse: collapse; width: 100%; }
td { padding: 0 8px; white-space: pre; }
td.line-num { color: #858585; text-align: right; user-select: none; width: 1%; border-right: 1px solid #333; }
tr.covered td { background: rgba(0, 128, 0, 0.2); }
tr.uncovered td { background: rgba(255, 0, 0, 0.2); }
tr.partial td { background: rgba(255, 165, 0, 0.2); }
</style>
</head>
<body>
<div class="summary">
<h1>Templ Coverage Report</h1>
<p>Total: {{printf "%.1f" .TotalPercentage}}% ({{.TotalCovered}}/{{.TotalTotal}})</p>
</div>
<select id="file-select" onchange="showFile(this.value)">
{{range $i, $f := .Files}}<option value="file-{{$i}}">{{$f.Name}} — {{printf "%.1f" $f.Percentage}}%</option>
{{end}}</select>
{{range $i, $f := .Files}}<div class="file-view{{if eq $i 0}} active{{end}}" id="file-{{$i}}">
<table>
{{range .Lines}}<tr class="{{.Class}}"><td class="line-num">{{.Number}}</td><td>{{.Text}}</td></tr>
{{end}}</table>
</div>
{{end}}
<script>
function showFile(id) {
  document.querySelectorAll('.file-view').forEach(e => e.classList.remove('active'));
  document.getElementById(id).classList.add('active');
}
</script>
</body>
</html>`
```

Add imports: `"bytes"`, `htmltemplate "html/template"`, `"path/filepath"`, `"sort"`, `"strings"`, `"os"`.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./cmd/templ/coveragecmd/ -run TestHTMLReport -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/templ/coveragecmd/report.go cmd/templ/coveragecmd/report_test.go
git commit -m "feat(coverage): implement HTML report output"
```

---

## Task 8: Integration Test

**Files:**
- Create: `cmd/templ/coveragecmd/integration_test.go`

- [ ] **Step 1: Write end-to-end integration test**

This test generates a template with coverage, runs it to produce a profile, then tests all three report formats.

Create `cmd/templ/coveragecmd/integration_test.go`:
```go
package coveragecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportIntegration(t *testing.T) {
	// Create a profile and manifest that simulate a real workflow
	dir := t.TempDir()

	// Write a mock .templ source file for HTML report
	sourceFile := filepath.Join(dir, "template.templ")
	os.WriteFile(sourceFile, []byte("package test\n\ntempl Hello(name string) {\n\t<div>{ name }</div>\n}\n"), 0644)

	manifest := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			sourceFile: {
				{Line: 2, Col: 0},
				{Line: 3, Col: 1},
				{Line: 3, Col: 8},
			},
		},
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := manifest.Write(manifestPath); err != nil {
		t.Fatal(err)
	}

	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			sourceFile: {
				{Line: 2, Col: 0, Hits: 5, Type: "expression"},
				{Line: 3, Col: 1, Hits: 5, Type: "expression"},
				{Line: 3, Col: 8, Hits: 0, Type: "expression"},
			},
		},
	}
	profilePath := filepath.Join(dir, "coverage.json")
	if err := profile.Write(profilePath); err != nil {
		t.Fatal(err)
	}

	t.Run("terminal", func(t *testing.T) {
		var buf bytes.Buffer
		err := runReport(&buf, []string{"-i", profilePath, "-m", manifestPath})
		if err != nil {
			t.Fatal(err)
		}
		output := buf.String()
		if !strings.Contains(output, "66.7%") {
			t.Errorf("expected 66.7%% coverage (2/3), got:\n%s", output)
		}
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		err := runReport(&buf, []string{"-i", profilePath, "-m", manifestPath, "--json"})
		if err != nil {
			t.Fatal(err)
		}
		var report JSONReport
		if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.Total.Covered != 2 || report.Total.Total != 3 {
			t.Errorf("expected 2/3, got %d/%d", report.Total.Covered, report.Total.Total)
		}
	})

	t.Run("html", func(t *testing.T) {
		htmlPath := filepath.Join(dir, "report.html")
		var buf bytes.Buffer
		err := runReport(&buf, []string{"-i", profilePath, "-m", manifestPath, "--html", "-o", htmlPath})
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(htmlPath)
		if err != nil {
			t.Fatal(err)
		}
		html := string(data)
		if !strings.Contains(html, "template.templ") {
			t.Error("expected filename in HTML report")
		}
		if !strings.Contains(html, "covered") {
			t.Error("expected covered class in HTML report")
		}
	})
}
```

- [ ] **Step 2: Run test**

```bash
go test ./cmd/templ/coveragecmd/ -run TestReportIntegration -v
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/templ/coveragecmd/integration_test.go
git commit -m "test(coverage): add report integration test"
```

---

## Task 9: Update Documentation

**Files:**
- Modify: `docs/docs/09-developer-tools/07-coverage.md`

- [ ] **Step 1: Update coverage documentation**

Add a "Reports" section to `docs/docs/09-developer-tools/07-coverage.md` after the "Merge Profiles" section:

```markdown
### Generate Reports

Generate a coverage manifest alongside instrumented templates:

```bash
templ generate --coverage --coverage-manifest=coverage-manifest.json
```

View coverage in the terminal:

```bash
templ coverage report -i=coverage/*.json -m=coverage-manifest.json
```

Generate an HTML report:

```bash
templ coverage report -i=coverage/*.json -m=coverage-manifest.json --html -o=coverage.html
```

Generate a JSON report for CI integration:

```bash
templ coverage report -i=coverage/*.json -m=coverage-manifest.json --json
```
```

- [ ] **Step 2: Commit**

```bash
git add docs/docs/09-developer-tools/07-coverage.md
git commit -m "docs: add coverage report command documentation"
```
