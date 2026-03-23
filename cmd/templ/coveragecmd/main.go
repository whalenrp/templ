package coveragecmd

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func Run(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: templ coverage <command>\nCommands: merge")
	}

	switch args[0] {
	case "merge":
		return runMerge(w, args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runMerge(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	inputPaths := fs.String("i", "", "Comma-separated input paths or glob patterns")
	outputPath := fs.String("o", "coverage.json", "Output file path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *inputPaths == "" {
		return fmt.Errorf("-i flag required: specify input coverage files")
	}

	// Expand input paths (handle globs and comma-separated lists)
	var files []string
	for _, pattern := range strings.Split(*inputPaths, ",") {
		pattern = strings.TrimSpace(pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return fmt.Errorf("no coverage files found matching: %s", *inputPaths)
	}

	// Load all profiles
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

	// Merge
	merged := MergeProfiles(profiles)

	// Write output
	if err := merged.Write(*outputPath); err != nil {
		return fmt.Errorf("failed to write merged profile: %w", err)
	}

	fmt.Fprintf(w, "Merged %d profiles into %s\n", len(profiles), *outputPath)
	return nil
}
