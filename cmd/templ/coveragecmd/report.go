package coveragecmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	htmltemplate "html/template"
	"io"
	"os"
	"sort"
	"strings"
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
	type fileStat struct {
		name    string
		covered int
		total   int
	}

	var stats []fileStat

	if manifest != nil {
		for filename, mPoints := range manifest.Files {
			covered := countCoveredAgainstManifest(profile.Files[filename], mPoints)
			stats = append(stats, fileStat{name: filename, covered: covered, total: len(mPoints)})
		}
		// Include profile-only files (stale manifest)
		for filename, pPoints := range profile.Files {
			if _, inManifest := manifest.Files[filename]; !inManifest {
				stats = append(stats, fileStat{name: filename, covered: countCovered(pPoints), total: -1})
			}
		}
	} else {
		for filename, pPoints := range profile.Files {
			stats = append(stats, fileStat{name: filename, covered: countCovered(pPoints), total: -1})
		}
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].name < stats[j].name
	})

	maxLen := len("total")
	for _, s := range stats {
		if len(s.name) > maxLen {
			maxLen = len(s.name)
		}
	}

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

	// Collect all files from both manifest and profile
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

	var filenames []string
	for f := range allFiles {
		filenames = append(filenames, f)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		fd := fileData{Name: filename}

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

		// Build line-level coverage status
		lineCovered := make(map[uint32]bool)
		lineUncovered := make(map[uint32]bool)

		if manifest != nil {
			coveredSet := make(map[Position]bool)
			for _, p := range profile.Files[filename] {
				if p.Hits > 0 {
					coveredSet[Position{Line: p.Line, Col: p.Col}] = true
				}
			}
			for _, mp := range manifest.Files[filename] {
				if coveredSet[Position{Line: mp.Line, Col: mp.Col}] {
					lineCovered[mp.Line] = true
				} else {
					lineUncovered[mp.Line] = true
				}
			}
		} else {
			for _, p := range profile.Files[filename] {
				if p.Hits > 0 {
					lineCovered[p.Line] = true
				} else {
					lineUncovered[p.Line] = true
				}
			}
		}

		lines := strings.Split(strings.TrimRight(string(source), "\n"), "\n")
		for i, line := range lines {
			lineNum := uint32(i + 1)
			li := lineInfo{Number: i + 1, Text: line}
			hasCov := lineCovered[lineNum]
			hasUncov := lineUncovered[lineNum]
			if hasCov && hasUncov {
				li.Class = "partial"
			} else if hasCov {
				li.Class = "covered"
			} else if hasUncov {
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
{{end}}<script>
function showFile(id) {
  document.querySelectorAll('.file-view').forEach(function(e) { e.classList.remove('active'); });
  document.getElementById(id).classList.add('active');
}
</script>
</body>
</html>`

// JSONReport is the output structure for JSON coverage reports.
type JSONReport struct {
	Version string                       `json:"version"`
	Total   JSONReportSummary            `json:"total"`
	Files   map[string]JSONReportSummary `json:"files"`
}

// JSONReportSummary contains coverage statistics for a file or total.
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
		summary := JSONReportSummary{}
		var covered int

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
