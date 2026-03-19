package coveragecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
			"c.templ": {{Line: 1, Col: 0}},
		},
	}

	var buf bytes.Buffer
	if err := generateTerminalReport(&buf, profile, manifest); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "a.templ") || !strings.Contains(output, "50.0%") {
		t.Errorf("expected a.templ at 50%%, got:\n%s", output)
	}
	if !strings.Contains(output, "b.templ") || !strings.Contains(output, "100.0%") {
		t.Errorf("expected b.templ at 100%%, got:\n%s", output)
	}
	if !strings.Contains(output, "c.templ") || !strings.Contains(output, "0.0%") {
		t.Errorf("expected c.templ at 0%%, got:\n%s", output)
	}
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
	if !strings.Contains(output, "a.templ") {
		t.Errorf("expected a.templ in output, got:\n%s", output)
	}
	if strings.Contains(output, "%") {
		t.Errorf("expected no percentages without manifest, got:\n%s", output)
	}
}

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

func TestHTMLReport(t *testing.T) {
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
