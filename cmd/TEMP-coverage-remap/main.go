// TEMP: prototype to validate source-map based coverage remapping.
// Reads a standard go test -coverprofile and remaps .go positions to .templ
// positions using the templ generator's source map.
//
// Key insight: instead of looking up source map at coverage block start positions
// (which mostly fall on boilerplate), we iterate each source map entry and find
// which coverage block CONTAINS that Go position.
//
// Usage: go run ./cmd/TEMP-coverage-remap -profile=cov.out template.templ
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/a-h/templ/generator"
	parser "github.com/a-h/templ/parser/v2"
	"golang.org/x/tools/cover"
)

type smEntry struct {
	sm        *parser.SourceMap
	templPath string
}

func main() {
	profilePath := flag.String("profile", "coverage.out", "go test -coverprofile output")
	debug := flag.Bool("debug", false, "print debug info")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: TEMP-coverage-remap -profile=cov.out <template.templ> ...")
		os.Exit(1)
	}

	sourceMaps := map[string]smEntry{}
	for _, templPath := range flag.Args() {
		sm, err := sourceMapFor(templPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", templPath, err)
			continue
		}
		goBase := strings.TrimSuffix(templPath, ".templ") + "_templ.go"
		sourceMaps[goBase] = smEntry{sm, templPath}
		fmt.Fprintf(os.Stderr, "loaded source map for %s (%d target→source entries)\n",
			templPath, countEntries(sm))
	}

	profiles, err := cover.ParseProfiles(*profilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse profile:", err)
		os.Exit(1)
	}

	if *debug {
		for goBase, entry := range sourceMaps {
			printDebug(goBase, entry, profiles)
		}
		return
	}

	printRemapped(os.Stdout, profiles, sourceMaps)
}

// blockIndex builds a fast lookup: for a given Go line, which blocks overlap it?
// Blocks are sorted by start line within each profile.
type blockIndex struct {
	profile *cover.Profile
	// sorted slice of blocks for binary search
	blocks []cover.ProfileBlock
}

func buildBlockIndex(p *cover.Profile) *blockIndex {
	blocks := make([]cover.ProfileBlock, len(p.Blocks))
	copy(blocks, p.Blocks)
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].StartLine != blocks[j].StartLine {
			return blocks[i].StartLine < blocks[j].StartLine
		}
		return blocks[i].StartCol < blocks[j].StartCol
	})
	return &blockIndex{profile: p, blocks: blocks}
}

// hitCount returns the hit count for a block containing (line, col), or -1 if none.
func (idx *blockIndex) hitCount(line, col int) int {
	for _, b := range idx.blocks {
		if b.StartLine > line {
			break
		}
		// Check if (line, col) is within [start, end).
		afterStart := b.StartLine < line || (b.StartLine == line && b.StartCol <= col)
		beforeEnd := b.EndLine > line || (b.EndLine == line && b.EndCol > col)
		if afterStart && beforeEnd {
			return b.Count
		}
	}
	return -1
}

func printDebug(goBase string, entry smEntry, profiles []*cover.Profile) {
	// Find the matching profile.
	var idx *blockIndex
	for _, p := range profiles {
		if strings.HasSuffix(p.FileName, fileBaseName(goBase)) {
			idx = buildBlockIndex(p)
			break
		}
	}
	if idx == nil {
		fmt.Fprintf(os.Stderr, "no profile found for %s\n", goBase)
		return
	}

	fmt.Fprintf(os.Stderr, "\n=== DEBUG: remapping for %s ===\n", goBase)
	sm := entry.sm
	lines := make([]int, 0)
	for line := range sm.TargetLinesToSource {
		lines = append(lines, int(line))
	}
	sort.Ints(lines)

	for _, goLine := range lines {
		cols := make([]int, 0)
		for col := range sm.TargetLinesToSource[uint32(goLine)] {
			cols = append(cols, int(col))
		}
		sort.Ints(cols)
		for _, goCol := range cols {
			hit := idx.hitCount(goLine, goCol)
			srcPos := sm.TargetLinesToSource[uint32(goLine)][uint32(goCol)]
			fmt.Fprintf(os.Stderr, "  go %d:%d → templ %d:%d  hit=%d\n",
				goLine, goCol, srcPos.Line, srcPos.Col, hit)
		}
	}
}

func printRemapped(w io.Writer, profiles []*cover.Profile, sourceMaps map[string]smEntry) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	fmt.Fprintln(bw, "mode: set")

	// For each templ file, build a map: templ line → max hit count seen.
	type templLine struct {
		file string
		line uint32
		col  uint32
	}

	var mapped, unmapped int

	for _, p := range profiles {
		entry, found := findSourceMap(p.FileName, sourceMaps)
		if !found {
			// Pass through non-templ files unchanged.
			for _, b := range p.Blocks {
				fmt.Fprintf(bw, "%s:%d.%d,%d.%d %d %d\n",
					p.FileName, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt, b.Count)
			}
			continue
		}

		idx := buildBlockIndex(p)
		sm := entry.sm

		// Collect all target (Go) positions from the source map, look them up in
		// the coverage index, and emit the corresponding templ positions.
		// Use map to deduplicate/aggregate: templ position → count.
		type templPos struct {
			line, col uint32
		}
		counts := map[templPos]int{}

		for goLine, cols := range sm.TargetLinesToSource {
			for goCol, srcPos := range cols {
				hit := idx.hitCount(int(goLine), int(goCol))
				if hit < 0 {
					unmapped++
					continue
				}
				tp := templPos{srcPos.Line, srcPos.Col}
				if existing, ok := counts[tp]; !ok || hit > existing {
					counts[tp] = hit
				}
				mapped++
			}
		}

		// Sort and emit.
		type entry2 struct {
			pos   templPos
			count int
		}
		entries := make([]entry2, 0, len(counts))
		for pos, count := range counts {
			entries = append(entries, entry2{pos, count})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].pos.line != entries[j].pos.line {
				return entries[i].pos.line < entries[j].pos.line
			}
			return entries[i].pos.col < entries[j].pos.col
		})

		for _, e := range entries {
			fmt.Fprintf(bw, "%s:%d.%d,%d.%d %d %d\n",
				entry.templPath,
				e.pos.line, e.pos.col,
				e.pos.line, e.pos.col,
				1, e.count)
		}
	}

	fmt.Fprintf(os.Stderr, "remapped: %d source map entries mapped, %d not in any block\n", mapped, unmapped)
}

func findSourceMap(profileFileName string, sourceMaps map[string]smEntry) (smEntry, bool) {
	base := fileBaseName(profileFileName)
	for goBase, entry := range sourceMaps {
		if strings.HasSuffix(goBase, "/"+base) || goBase == base {
			return entry, true
		}
	}
	return smEntry{}, false
}

func fileBaseName(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func sourceMapFor(templPath string) (*parser.SourceMap, error) {
	tf, err := parser.Parse(templPath)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	out, err := generator.Generate(tf, io.Discard, generator.WithFileName(templPath))
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	return out.SourceMap, nil
}

func countEntries(sm *parser.SourceMap) int {
	n := 0
	for _, cols := range sm.TargetLinesToSource {
		n += len(cols)
	}
	return n
}
