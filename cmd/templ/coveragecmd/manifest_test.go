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
