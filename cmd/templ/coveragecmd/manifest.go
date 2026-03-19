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
	Version string                     `json:"version"`
	Files   map[string][]ManifestPoint `json:"files"`
}

// LoadManifest reads a coverage manifest from a JSON file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to decode manifest %s: %w", path, err)
	}

	if m.Version != "1" {
		return nil, fmt.Errorf("unsupported manifest version %q in %s, expected \"1\"", m.Version, path)
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
