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
