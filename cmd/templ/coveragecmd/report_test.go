package coveragecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// buildHits constructs a profileHits from (filename, line1indexed, col1indexed, count) tuples.
func buildHits(entries ...any) profileHits {
	h := make(profileHits)
	for i := 0; i < len(entries); i += 4 {
		file := entries[i].(string)
		line := entries[i+1].(int)
		col := entries[i+2].(int)
		count := entries[i+3].(uint32)
		if h[file] == nil {
			h[file] = make(map[profilePos]uint32)
		}
		h[file][profilePos{line, col}]++
		h[file][profilePos{line, col}] = count
	}
	return h
}

func TestTerminalReport(t *testing.T) {
	// Manifest uses 0-indexed; profile uses 1-indexed (line+1, col+1).
	hits := buildHits(
		"a.templ", 2, 1, uint32(3), // manifest line=1,col=0 → profile 2.1
		"b.templ", 2, 1, uint32(1), // manifest line=1,col=0 → profile 2.1
	)
	manifest := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			"a.templ": {{Line: 1, Col: 0}, {Line: 2, Col: 0}},
			"b.templ": {{Line: 1, Col: 0}},
		},
	}

	var buf bytes.Buffer
	if err := generateTerminalReport(&buf, hits, manifest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "a.templ") {
		t.Errorf("expected a.templ in output:\n%s", out)
	}
	if !strings.Contains(out, "b.templ") {
		t.Errorf("expected b.templ in output:\n%s", out)
	}
	if !strings.Contains(out, "total") {
		t.Errorf("expected total line in output:\n%s", out)
	}
}

func TestTerminalReport_NoManifest(t *testing.T) {
	hits := buildHits("a.templ", 2, 1, uint32(5))
	var buf bytes.Buffer
	if err := generateTerminalReport(&buf, hits, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "a.templ") {
		t.Errorf("expected a.templ in output:\n%s", out)
	}
	if !strings.Contains(out, "points covered") {
		t.Errorf("expected 'points covered' without manifest:\n%s", out)
	}
}

func TestJSONReport(t *testing.T) {
	hits := buildHits("a.templ", 2, 1, uint32(3))
	manifest := &Manifest{
		Version: "1",
		Files: map[string][]ManifestPoint{
			"a.templ": {{Line: 1, Col: 0}, {Line: 2, Col: 0}},
		},
	}

	var buf bytes.Buffer
	if err := generateJSONReport(&buf, hits, manifest, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.Version != "1" {
		t.Errorf("version = %q, want 1", report.Version)
	}
	if _, ok := report.Files["a.templ"]; !ok {
		t.Error("expected a.templ in JSON report")
	}
}

func TestLoadAndMerge(t *testing.T) {
	// Write two standard-format profiles to temp files and verify they merge.
	dir := t.TempDir()
	writeProfile := func(name, content string) string {
		path := dir + "/" + name
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	p1 := writeProfile("p1.out", "mode: count\na.templ:2.1,2.1 1 3\n")
	p2 := writeProfile("p2.out", "mode: count\na.templ:2.1,2.1 1 2\nb.templ:5.3,5.3 1 1\n")

	merged, err := loadAndMerge([]string{p1, p2})
	if err != nil {
		t.Fatalf("loadAndMerge: %v", err)
	}

	if merged["a.templ"][profilePos{2, 1}] != 5 {
		t.Errorf("a.templ 2.1 count = %d, want 5", merged["a.templ"][profilePos{2, 1}])
	}
	if merged["b.templ"][profilePos{5, 3}] != 1 {
		t.Errorf("b.templ 5.3 count = %d, want 1", merged["b.templ"][profilePos{5, 3}])
	}
}

func TestIsCovered(t *testing.T) {
	hits := buildHits(
		"t.templ", 4, 2, uint32(1), // profile 1-indexed: line=4,col=2 → manifest line=3,col=1
	)
	if !isCovered(hits, "t.templ", 3, 1) { // manifest 0-indexed
		t.Error("expected covered")
	}
	if isCovered(hits, "t.templ", 3, 2) {
		t.Error("unexpected covered for different col")
	}
	if isCovered(hits, "other.templ", 3, 1) {
		t.Error("unexpected covered for different file")
	}
}
