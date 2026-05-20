package coveragecmd

import (
	"fmt"
	"io"
)

func Run(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: templ coverage report [flags]")
	}

	switch args[0] {
	case "report":
		return runReport(w, args[1:])
	default:
		return fmt.Errorf("unknown command %q — only 'report' is supported\n"+
			"To merge profiles use: go tool covdata merge (Go 1.20+) or gocovmerge", args[0])
	}
}
