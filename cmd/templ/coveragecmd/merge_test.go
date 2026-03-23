package coveragecmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid profile",
			content: `{
				"version": "1",
				"mode": "count",
				"files": {
					"test.templ": [
						{"line": 1, "col": 0, "hits": 5, "type": "expression"}
					]
				}
			}`,
			wantErr: false,
		},
		{
			name: "invalid version",
			content: `{
				"version": "2",
				"mode": "count",
				"files": {}
			}`,
			wantErr:   true,
			errSubstr: "unsupported version",
		},
		{
			name:      "invalid JSON",
			content:   `{invalid json}`,
			wantErr:   true,
			errSubstr: "decode",
		},
		{
			name:      "nonexistent file",
			content:   "",
			wantErr:   true,
			errSubstr: "read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.content != "" {
				// Create temp file
				tmpfile, err := os.CreateTemp("", "profile*.json")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpfile.Name())
				path = tmpfile.Name()

				if _, err := tmpfile.Write([]byte(tt.content)); err != nil {
					t.Fatal(err)
				}
				if err := tmpfile.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				path = filepath.Join(os.TempDir(), "nonexistent-profile.json")
			}

			profile, err := LoadProfile(path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadProfile() expected error containing %q, got nil", tt.errSubstr)
				} else if tt.errSubstr != "" && !contains(err.Error(), tt.errSubstr) {
					t.Errorf("LoadProfile() error = %v, want substring %q", err, tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("LoadProfile() unexpected error = %v", err)
				}
				if profile == nil {
					t.Error("LoadProfile() returned nil profile")
				}
				if profile != nil && profile.Version != "1" {
					t.Errorf("LoadProfile() version = %q, want %q", profile.Version, "1")
				}
			}
		})
	}
}

func TestMergeProfiles(t *testing.T) {
	tests := []struct {
		name     string
		profiles []*Profile
		want     *Profile
	}{
		{
			name:     "empty profiles",
			profiles: []*Profile{},
			want: &Profile{
				Version: "1",
				Mode:    "count",
				Files:   map[string][]CoveragePoint{},
			},
		},
		{
			name: "single profile",
			profiles: []*Profile{
				{
					Version: "1",
					Mode:    "count",
					Files: map[string][]CoveragePoint{
						"test.templ": {
							{Line: 1, Col: 0, Hits: 5, Type: "expression"},
						},
					},
				},
			},
			want: &Profile{
				Version: "1",
				Mode:    "count",
				Files: map[string][]CoveragePoint{
					"test.templ": {
						{Line: 1, Col: 0, Hits: 5, Type: "expression"},
					},
				},
			},
		},
		{
			name: "merge same position",
			profiles: []*Profile{
				{
					Version: "1",
					Mode:    "count",
					Files: map[string][]CoveragePoint{
						"test.templ": {
							{Line: 1, Col: 0, Hits: 5, Type: "expression"},
						},
					},
				},
				{
					Version: "1",
					Mode:    "count",
					Files: map[string][]CoveragePoint{
						"test.templ": {
							{Line: 1, Col: 0, Hits: 3, Type: "expression"},
						},
					},
				},
			},
			want: &Profile{
				Version: "1",
				Mode:    "count",
				Files: map[string][]CoveragePoint{
					"test.templ": {
						{Line: 1, Col: 0, Hits: 8, Type: "expression"},
					},
				},
			},
		},
		{
			name: "merge different positions",
			profiles: []*Profile{
				{
					Version: "1",
					Mode:    "count",
					Files: map[string][]CoveragePoint{
						"test.templ": {
							{Line: 1, Col: 0, Hits: 5, Type: "expression"},
						},
					},
				},
				{
					Version: "1",
					Mode:    "count",
					Files: map[string][]CoveragePoint{
						"test.templ": {
							{Line: 2, Col: 0, Hits: 3, Type: "expression"},
						},
					},
				},
			},
			want: &Profile{
				Version: "1",
				Mode:    "count",
				Files: map[string][]CoveragePoint{
					"test.templ": {
						{Line: 1, Col: 0, Hits: 5, Type: "expression"},
						{Line: 2, Col: 0, Hits: 3, Type: "expression"},
					},
				},
			},
		},
		{
			name: "merge different files",
			profiles: []*Profile{
				{
					Version: "1",
					Mode:    "count",
					Files: map[string][]CoveragePoint{
						"a.templ": {
							{Line: 1, Col: 0, Hits: 5, Type: "expression"},
						},
					},
				},
				{
					Version: "1",
					Mode:    "count",
					Files: map[string][]CoveragePoint{
						"b.templ": {
							{Line: 1, Col: 0, Hits: 3, Type: "expression"},
						},
					},
				},
			},
			want: &Profile{
				Version: "1",
				Mode:    "count",
				Files: map[string][]CoveragePoint{
					"a.templ": {
						{Line: 1, Col: 0, Hits: 5, Type: "expression"},
					},
					"b.templ": {
						{Line: 1, Col: 0, Hits: 3, Type: "expression"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeProfiles(tt.profiles)
			if !profilesEqual(got, tt.want) {
				t.Errorf("MergeProfiles() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestProfile_Write(t *testing.T) {
	profile := &Profile{
		Version: "1",
		Mode:    "count",
		Files: map[string][]CoveragePoint{
			"test.templ": {
				{Line: 1, Col: 0, Hits: 5, Type: "expression"},
				{Line: 2, Col: 4, Hits: 3, Type: "expression"},
			},
		},
	}

	tmpfile, err := os.CreateTemp("", "profile*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	if err := profile.Write(tmpfile.Name()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Read back and verify
	loaded, err := LoadProfile(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadProfile() error = %v", err)
	}

	if !profilesEqual(loaded, profile) {
		t.Errorf("Write/Load roundtrip failed: got %+v, want %+v", loaded, profile)
	}

	// Verify it's valid JSON
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	var checkJSON map[string]interface{}
	if err := json.Unmarshal(data, &checkJSON); err != nil {
		t.Errorf("Write() produced invalid JSON: %v", err)
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func profilesEqual(a, b *Profile) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Version != b.Version || a.Mode != b.Mode {
		return false
	}
	if len(a.Files) != len(b.Files) {
		return false
	}
	for filename, aPoints := range a.Files {
		bPoints, ok := b.Files[filename]
		if !ok || len(aPoints) != len(bPoints) {
			return false
		}
		// Create maps for comparison (order doesn't matter)
		aMap := make(map[Position]CoveragePoint)
		for _, p := range aPoints {
			aMap[Position{Line: p.Line, Col: p.Col}] = p
		}
		bMap := make(map[Position]CoveragePoint)
		for _, p := range bPoints {
			bMap[Position{Line: p.Line, Col: p.Col}] = p
		}
		if len(aMap) != len(bMap) {
			return false
		}
		for pos, aPoint := range aMap {
			bPoint, ok := bMap[pos]
			if !ok {
				return false
			}
			if aPoint.Hits != bPoint.Hits {
				return false
			}
			if aPoint.Type != bPoint.Type {
				return false
			}
		}
	}
	return true
}
