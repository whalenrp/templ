// TEMP: prototype validation test for //line directive coverage approach.
package templinedirective

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/cover"
)

// TestTEMP_LineDirecExerciseTrue exercises only the true branch of Greet.
// Intended to be run in isolation via subprocess to get a clean coverage profile.
func TestTEMP_LineDirecExerciseTrue(t *testing.T) {
	var sb strings.Builder
	if err := Greet(true).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "Hello") {
		t.Errorf("unexpected output: %s", sb.String())
	}
}

// TestTEMP_LineDirecCoverage is the validation test.
//
// It runs TestTEMP_LineDirecExerciseTrue in a subprocess with go test -coverprofile,
// then asserts on the resulting profile to verify that //line directives cause coverage
// line numbers to be in templ coordinate space rather than generated Go file coordinate
// space, and that branch attribution is correct.
//
// The template has ~10 lines. The generated Go file has ~35+ lines of boilerplate
// before the first template construct. Therefore:
//
//   - If //line directives work: the if-show block appears at line 4 (templ coordinate).
//   - If //line directives don't work: the if-show block appears at line 33 (Go file
//     coordinate), and findBlockAtLine(4) returns nil, failing the test.
//
// This test also checks for backwards coverage blocks (EndLine < StartLine), which are
// a side effect of //line directives that reset the virtual line counter backwards.
// Backwards blocks occur when a //line directive inside a branch body jumps the counter
// to an earlier line (e.g. //line template.templ:7 while the counter is at virtual line 9).
func TestTEMP_LineDirecCoverage(t *testing.T) {
	if os.Getenv("TEMP_LINEDIREC_CHILD") != "" {
		// We are the subprocess; the exercise test does the actual work.
		return
	}

	profPath := filepath.Join(t.TempDir(), "coverage.out")

	// Use the real Go binary, not the monorepo wrapper (which requires a git root).
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")

	// Run go test on this package, exercising only the true branch.
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)

	cmd := exec.Command(goBin, "test",
		"-run=^TestTEMP_LineDirecExerciseTrue$",
		"-coverprofile="+profPath,
		".",
	)
	cmd.Dir = pkgDir
	cmd.Env = append(os.Environ(), "TEMP_LINEDIREC_CHILD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test subprocess failed: %v\n%s", err, out)
	}

	profiles, err := cover.ParseProfiles(profPath)
	if err != nil {
		t.Fatalf("parse profile: %v", err)
	}

	var p *cover.Profile
	for _, prof := range profiles {
		if strings.HasSuffix(prof.FileName, "template_templ.go") {
			p = prof
			break
		}
	}
	if p == nil {
		t.Fatal("no coverage profile found for template_templ.go; files in profile:", profileSummary(profiles))
	}

	t.Logf("coverage blocks: %s", describeBlocks(p.Blocks))

	// --- Assertion 1: //line directives redirect line numbers to templ coordinates ---
	//
	// The `if show {` statement is at real Go file line 33. With `//line template.templ:4`
	// immediately before it, the coverage block for the if-true entry should appear at
	// StartLine=4 (templ coordinate), not 33 (Go file coordinate).
	//
	// If //line does NOT work, no block will have StartLine=4, and these fail.

	trueBlock := findBlockAtLine(p.Blocks, 4)
	if trueBlock == nil {
		t.Errorf("no coverage block at StartLine=4 (templ 'if show' / true-branch entry):\n"+
			"  //line directives are not redirecting line numbers to templ coordinates.\n"+
			"  Got blocks at lines: %s", startLines(p.Blocks))
	}

	// The else entry point lands at virtual line 9 (natural count from line 4 through
	// the 5 boilerplate lines inside the true body). There is no way to pin the else
	// entry to a specific templ line using //line because `} else {` cannot be separated
	// from `}` on the prior line.
	elseBlock := findBlockAtLine(p.Blocks, 9)
	if elseBlock == nil {
		t.Errorf("no coverage block at StartLine=9 (else-branch entry, natural virtual line):\n"+
			"  Got blocks at lines: %s", startLines(p.Blocks))
	}

	// --- Assertion 2: branch attribution is correct ---
	//
	// We only called Greet(true), so:
	//   - true branch block (StartLine=4) must be covered (count > 0)
	//   - else branch block (StartLine=9) must NOT be covered (count == 0)

	if trueBlock != nil && trueBlock.Count == 0 {
		t.Errorf("block at line 4 (true branch): count=%d, want >0 after Greet(true)", trueBlock.Count)
	}
	if elseBlock != nil && elseBlock.Count != 0 {
		t.Errorf("block at line 9 (else branch): count=%d, want 0 after Greet(true) only", elseBlock.Count)
	}

	// --- Assertion 3: backwards blocks are detected ---
	//
	// //line directives inside branch bodies (e.g. //line template.templ:7 inside the
	// else body when the virtual counter is at line 9) cause the virtual line counter to
	// jump backwards, producing coverage blocks where EndLine < StartLine. These are
	// valid entries in the profile but malformed from the coverage tool's perspective.
	//
	// This assertion FAILS if backwards blocks are present, documenting this as a
	// known limitation of the //line approach when used inside branch bodies.

	var backwards []string
	for _, b := range p.Blocks {
		if b.EndLine < b.StartLine {
			backwards = append(backwards, fmt.Sprintf("%d.%d,%d.%d(count=%d)",
				b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.Count))
		}
	}
	if len(backwards) > 0 {
		t.Errorf("backwards coverage blocks detected (EndLine < StartLine): %v\n"+
			"  This happens when a //line directive inside a branch body resets the\n"+
			"  virtual line counter backwards. Blocks: %s",
			backwards, describeBlocks(p.Blocks))
	}

	// --- Assertion 4: file name remapping is a simple string substitution ---
	//
	// Because //line directives already put the correct line numbers in the profile,
	// converting template_templ.go → template.templ requires only a file name
	// substitution.

	if !strings.HasSuffix(p.FileName, "template_templ.go") {
		t.Errorf("profile FileName=%q: expected to end with template_templ.go", p.FileName)
	}
	remapped := strings.TrimSuffix(p.FileName, "template_templ.go") + "template.templ"
	if !strings.HasSuffix(remapped, "template.templ") {
		t.Errorf("remapped file name %q does not end with template.templ", remapped)
	}
}

func findBlockAtLine(blocks []cover.ProfileBlock, line int) *cover.ProfileBlock {
	for i := range blocks {
		if blocks[i].StartLine == line {
			return &blocks[i]
		}
	}
	return nil
}

func startLines(blocks []cover.ProfileBlock) string {
	parts := make([]string, len(blocks))
	for i, b := range blocks {
		parts[i] = fmt.Sprintf("%d", b.StartLine)
	}
	return strings.Join(parts, ", ")
}

func describeBlocks(blocks []cover.ProfileBlock) string {
	parts := make([]string, len(blocks))
	for i, b := range blocks {
		parts[i] = fmt.Sprintf("%d.%d,%d.%d(n=%d,c=%d)",
			b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt, b.Count)
	}
	return strings.Join(parts, " ")
}

func profileSummary(profiles []*cover.Profile) string {
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.FileName
	}
	return strings.Join(names, ", ")
}
