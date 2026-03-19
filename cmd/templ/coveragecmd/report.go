package coveragecmd

import (
	"flag"
	"fmt"
	"io"
)

func runReport(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	inputPaths := fs.String("i", "", "Comma-separated input coverage profiles or glob patterns")
	manifestPath := fs.String("m", "", "Coverage manifest file")
	htmlOutput := fs.Bool("html", false, "Generate HTML report")
	jsonOutput := fs.Bool("json", false, "Generate JSON report")
	outputPath := fs.String("o", "", "Output file path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *inputPaths == "" {
		return fmt.Errorf("-i flag required: specify input coverage profiles")
	}

	// Load and merge profiles
	files, err := expandInputPaths(*inputPaths)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no coverage profiles found matching: %s", *inputPaths)
	}

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
	merged := MergeProfiles(profiles)

	// Load manifest (optional)
	var manifest *Manifest
	if *manifestPath != "" {
		manifest, err = LoadManifest(*manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}
	} else {
		fmt.Fprintln(w, "Warning: No manifest provided (-m); coverage percentages unavailable.")
	}

	// Dispatch to format-specific generator
	switch {
	case *htmlOutput:
		return generateHTMLReport(w, merged, manifest, *outputPath)
	case *jsonOutput:
		return generateJSONReport(w, merged, manifest, *outputPath)
	default:
		return generateTerminalReport(w, merged, manifest)
	}
}

func generateTerminalReport(w io.Writer, profile *Profile, manifest *Manifest) error {
	return fmt.Errorf("terminal report not yet implemented")
}

func generateHTMLReport(w io.Writer, profile *Profile, manifest *Manifest, outputPath string) error {
	return fmt.Errorf("HTML report not yet implemented")
}

func generateJSONReport(w io.Writer, profile *Profile, manifest *Manifest, outputPath string) error {
	return fmt.Errorf("JSON report not yet implemented")
}
