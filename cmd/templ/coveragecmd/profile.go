package coveragecmd

import (
	"encoding/json"
	"fmt"
	"os"
)

// CoveragePoint represents a single coverage measurement point
type CoveragePoint struct {
	Line uint32 `json:"line"`
	Col  uint32 `json:"col"`
	Hits uint32 `json:"hits"`
	Type string `json:"type"`
}

// Profile represents a complete coverage profile
type Profile struct {
	Version string                     `json:"version"`
	Mode    string                     `json:"mode"`
	Files   map[string][]CoveragePoint `json:"files"`
}

// LoadProfile reads and validates a coverage profile from a JSON file
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile: %w", err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to decode profile: %w", err)
	}

	if profile.Version != "1" {
		return nil, fmt.Errorf("unsupported version %q, expected \"1\"", profile.Version)
	}

	return &profile, nil
}

// Write saves the profile to a JSON file
func (p *Profile) Write(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}

	return nil
}
