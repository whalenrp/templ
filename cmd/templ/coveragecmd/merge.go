package coveragecmd

// Position represents a unique location in a file
type Position struct {
	Line uint32
	Col  uint32
}

// MergeProfiles combines multiple coverage profiles by summing hit counts
func MergeProfiles(profiles []*Profile) *Profile {
	result := &Profile{
		Version: "1",
		Mode:    "count",
		Files:   make(map[string][]CoveragePoint),
	}

	// Use intermediate map for efficient merging
	type filePos struct {
		filename string
		pos      Position
	}
	merged := make(map[filePos]*CoveragePoint)

	// Collect all coverage points
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		for filename, points := range profile.Files {
			for _, point := range points {
				key := filePos{
					filename: filename,
					pos:      Position{Line: point.Line, Col: point.Col},
				}
				if existing, found := merged[key]; found {
					// Sum hits for same position, preserve type from first occurrence
					existing.Hits += point.Hits
				} else {
					// New position - store full point
					pointCopy := point
					merged[key] = &pointCopy
				}
			}
		}
	}

	// Convert back to profile format
	for key, point := range merged {
		result.Files[key.filename] = append(result.Files[key.filename], *point)
	}

	return result
}
