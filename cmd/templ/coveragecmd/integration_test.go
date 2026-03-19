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
		err := runReport(&buf, []string{"-i", profilePath, "-m", manifestPath, "-json"})
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
		err := runReport(&buf, []string{"-i", profilePath, "-m", manifestPath, "-html", "-o", htmlPath})
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
