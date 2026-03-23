package coveragecmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func Run(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: templ coverage <command>\nCommands: report")
	}

	switch args[0] {
	case "report":
		return runReport(w, args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// expandInputPaths splits comma-separated patterns and expands globs.
func expandInputPaths(inputPaths string) ([]string, error) {
	var files []string
	for _, pattern := range strings.Split(inputPaths, ",") {
		pattern = strings.TrimSpace(pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}
	return files, nil
}
