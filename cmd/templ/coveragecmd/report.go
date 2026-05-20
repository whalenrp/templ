package coveragecmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	htmltemplate "html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/cover"
)

func runReport(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	inputPaths := fs.String("i", "", "Comma-separated coverage profiles or glob patterns")
	manifestPath := fs.String("m", "", "Coverage manifest file (for total construct count)")
	sourceDir := fs.String("source-dir", ".", "Root directory for resolving .templ source files")
	htmlOutput := fs.Bool("html", false, "Generate HTML report")
	jsonOutput := fs.Bool("json", false, "Generate JSON report")
	outputPath := fs.String("o", "", "Output file path")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inputPaths == "" {
		return fmt.Errorf("-i flag required: specify input coverage profiles")
	}

	files, err := expandInputPaths(*inputPaths)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no coverage profiles found matching: %s", *inputPaths)
	}

	merged, err := loadAndMerge(files)
	if err != nil {
		return err
	}

	var manifest *Manifest
	if *manifestPath != "" {
		manifest, err = LoadManifest(*manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}
	} else {
		fmt.Fprintln(w, "Warning: No manifest provided (-m); coverage percentages unavailable.")
	}

	switch {
	case *htmlOutput:
		return generateHTMLReport(w, merged, manifest, *outputPath, *sourceDir)
	case *jsonOutput:
		return generateJSONReport(w, merged, manifest, *outputPath)
	default:
		return generateTerminalReport(w, merged, manifest)
	}
}

// profileHits is a flat representation of hit counts keyed by file and position.
// Line/col are as written in the profile (1-indexed), matching 0-indexed parser
// positions after subtracting 1.
type profileHits map[string]map[profilePos]uint32

type profilePos struct{ line, col int }

// loadAndMerge parses all profile files and sums hit counts for matching points.
func loadAndMerge(files []string) (profileHits, error) {
	merged := make(profileHits)
	for _, path := range files {
		profiles, err := cover.ParseProfiles(path)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		for _, p := range profiles {
			if merged[p.FileName] == nil {
				merged[p.FileName] = make(map[profilePos]uint32)
			}
			for _, b := range p.Blocks {
				pos := profilePos{b.StartLine, b.StartCol}
				merged[p.FileName][pos] += uint32(b.Count)
			}
		}
	}
	return merged, nil
}

// isCovered reports whether a manifest point (0-indexed) was hit.
// The profile stores 1-indexed positions, so we add 1 when looking up.
func isCovered(hits profileHits, filename string, line, col uint32) bool {
	if m, ok := hits[filename]; ok {
		return m[profilePos{int(line) + 1, int(col) + 1}] > 0
	}
	return false
}

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

// --- Terminal report ---

func generateTerminalReport(w io.Writer, hits profileHits, manifest *Manifest) error {
	type fileStat struct {
		name    string
		covered int
		total   int // -1 if no manifest data
	}

	var stats []fileStat
	allFiles := collectFiles(hits, manifest)

	for _, filename := range allFiles {
		covered, total := countForFile(hits, manifest, filename)
		stats = append(stats, fileStat{filename, covered, total})
	}

	maxLen := len("total")
	for _, s := range stats {
		if len(s.name) > maxLen {
			maxLen = len(s.name)
		}
	}

	totalCovered, totalTotal := 0, 0
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

	if manifest != nil {
		fmt.Fprintf(w, "%-*s  %5.1f%%  (%d/%d)\n", maxLen, "total",
			percentage(totalCovered, totalTotal), totalCovered, totalTotal)
	} else {
		fmt.Fprintf(w, "%-*s  %d points covered\n", maxLen, "total", totalCovered)
	}
	return nil
}

// --- HTML report ---

func generateHTMLReport(w io.Writer, hits profileHits, manifest *Manifest, outputPath, sourceDir string) error {
	if outputPath == "" {
		outputPath = "coverage.html"
	}

	type lineInfo struct {
		Number int
		Text   string
		Class  string
	}
	type fileData struct {
		Name       string
		Lines      []lineInfo
		Covered    int
		Total      int
		Percentage float64
		Available  bool
	}

	allFiles := collectFiles(hits, manifest)
	var filesData []fileData
	totalCovered, totalTotal := 0, 0

	for _, filename := range allFiles {
		covered, total := countForFile(hits, manifest, filename)
		totalCovered += covered
		if total >= 0 {
			totalTotal += total
		}

		fd := fileData{
			Name:       filename,
			Covered:    covered,
			Total:      total,
			Percentage: percentage(covered, total),
		}

		sourcePath := filepath.Join(sourceDir, filename)
		source, err := os.ReadFile(sourcePath)
		if err != nil {
			fd.Lines = []lineInfo{{Number: 1, Text: "Source not available"}}
			filesData = append(filesData, fd)
			continue
		}
		fd.Available = true

		// Build per-display-line coverage class.
		// Manifest points are 0-indexed; display lines are 1-indexed.
		lineCovered := make(map[uint32]bool)
		lineUncovered := make(map[uint32]bool)

		if manifest != nil {
			for _, mp := range manifest.Files[filename] {
				displayLine := mp.Line + 1
				if isCovered(hits, filename, mp.Line, mp.Col) {
					lineCovered[displayLine] = true
				} else {
					lineUncovered[displayLine] = true
				}
			}
		} else {
			for pos, count := range hits[filename] {
				displayLine := uint32(pos.line) // profile is 1-indexed, already display line
				if count > 0 {
					lineCovered[displayLine] = true
				} else {
					lineUncovered[displayLine] = true
				}
			}
		}

		for i, line := range strings.Split(strings.TrimRight(string(source), "\n"), "\n") {
			lineNum := uint32(i + 1)
			li := lineInfo{Number: i + 1, Text: line}
			switch {
			case lineCovered[lineNum] && lineUncovered[lineNum]:
				li.Class = "partial"
			case lineCovered[lineNum]:
				li.Class = "covered"
			case lineUncovered[lineNum]:
				li.Class = "uncovered"
			}
			fd.Lines = append(fd.Lines, li)
		}
		filesData = append(filesData, fd)
	}

	tmplData := struct {
		Files           []fileData
		TotalCovered    int
		TotalTotal      int
		TotalPercentage float64
	}{filesData, totalCovered, totalTotal, percentage(totalCovered, totalTotal)}

	tmpl, err := htmltemplate.New("coverage").Parse(htmlReportTemplate)
	if err != nil {
		return fmt.Errorf("parse HTML template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return fmt.Errorf("render HTML: %w", err)
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// --- JSON report ---

type JSONReport struct {
	Version string                       `json:"version"`
	Total   JSONReportSummary            `json:"total"`
	Files   map[string]JSONReportSummary `json:"files"`
}

type JSONReportSummary struct {
	Covered    int     `json:"covered"`
	Total      int     `json:"total,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
}

func generateJSONReport(w io.Writer, hits profileHits, manifest *Manifest, outputPath string) error {
	report := JSONReport{Version: "1", Files: make(map[string]JSONReportSummary)}
	totalCovered, totalTotal := 0, 0

	for _, filename := range collectFiles(hits, manifest) {
		covered, total := countForFile(hits, manifest, filename)
		totalCovered += covered
		s := JSONReportSummary{Covered: covered}
		if total >= 0 {
			totalTotal += total
			s.Total = total
			s.Percentage = percentage(covered, total)
		}
		report.Files[filename] = s
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
		return fmt.Errorf("marshal JSON: %w", err)
	}
	if outputPath != "" {
		return os.WriteFile(outputPath, data, 0644)
	}
	_, err = w.Write(data)
	return err
}

// --- helpers ---

// collectFiles returns a sorted list of all files present in hits or manifest.
func collectFiles(hits profileHits, manifest *Manifest) []string {
	seen := make(map[string]bool)
	for f := range hits {
		seen[f] = true
	}
	if manifest != nil {
		for f := range manifest.Files {
			seen[f] = true
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// countForFile returns (covered, total) for a file.
// total is -1 when no manifest is available.
func countForFile(hits profileHits, manifest *Manifest, filename string) (covered, total int) {
	if manifest != nil {
		mPoints := manifest.Files[filename]
		total = len(mPoints)
		for _, mp := range mPoints {
			if isCovered(hits, filename, mp.Line, mp.Col) {
				covered++
			}
		}
		return covered, total
	}
	for _, count := range hits[filename] {
		if count > 0 {
			covered++
		}
	}
	return covered, -1
}

func percentage(covered, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
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
{{end}}<script>
function showFile(id) {
  document.querySelectorAll('.file-view').forEach(function(e) { e.classList.remove('active'); });
  document.getElementById(id).classList.add('active');
}
</script>
</body>
</html>`
