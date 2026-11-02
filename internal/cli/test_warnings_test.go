package cli_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"golang.org/x/tools/txtar"
)

// warningsCase is the command golden whose project holds one W1003 and one header its test reads with one W7101.
const warningsCase = "testdata/commands/test_warnings.txtar"

// wantExitWarnings is the exit code of more warnings than --max-warnings allows (CLI.md §2.5).
const wantExitWarnings = 4

// warningHead opens a warning's head line in check's text form.
const warningHead = "warning["

// CLI.md §3.5, §2.3, §2.5, DECISIONS 449: canon test prints its W1003 and W7101 first, -q keeps only their summary, --max-warnings counts them.
func TestTestReportsStaticWarnings(t *testing.T) {
	a, err := txtar.ParseFile(warningsCase)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args  []string
		code  int
		shown bool
	}{
		{args: []string{"test"}, code: 0, shown: true},
		{args: []string{"test", "--max-warnings", "2"}, code: 0, shown: true},
		{args: []string{"test", "--max-warnings", "0"}, code: wantExitWarnings, shown: true},
		{args: []string{"test", "-q"}, code: 0},
		{args: []string{"test", "-q", "--max-warnings", "1"}, code: wantExitWarnings},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			r := run(t, withArgs(a, tc.args))
			lines := strings.Split(r.stdout, "\n")
			got := []bool{
				slices.Contains(lines, warningAt(diag.W1003.Def(), "a/a.canon:8:7")),
				slices.Contains(lines, warningAt(diag.W7101.Def(), "a/data/a.h:2:1")),
				strings.HasPrefix(r.stdout, "0 errors, 2 warnings in 1 package (") ||
					strings.Contains(r.stdout, "\n\n0 errors, 2 warnings in 1 package ("),
				strings.Contains(r.stdout, ")\n\n1 passed, 0 failed ("),
			}
			want := []bool{tc.shown, tc.shown, true, true}
			if r.code != tc.code || !slices.Equal(got, want) {
				t.Errorf("exit %d (want %d), the two warnings, their summary, then the report %v (want %v):\n%s", r.code, tc.code, got, want, r.stdout)
			}
		})
	}
}

// CLI.md §3.5, API.md B5, DECISIONS 449: canon test's static block is canon check's output for the same selectors, every stage's warnings and project.canon's included.
func TestTestStaticBlockIsCheck(t *testing.T) {
	for _, tc := range []struct {
		file string
		max  string // a --max-warnings check exceeds: exit 4 in both, unless an error makes it 1
		code int
	}{
		{file: "testdata/commands/test_warnings_check.txtar", max: "3", code: wantExitWarnings},
		{file: "testdata/commands/test_warnings_pin.txtar", max: "4", code: wantExitWarnings},
		{file: "testdata/commands/test_warnings_broken.txtar", max: "0", code: wantExitError},
	} {
		t.Run(tc.file, func(t *testing.T) {
			a, err := txtar.ParseFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			checked := run(t, withArgs(a, []string{"check", "a", "--max-warnings", tc.max}))
			tested := run(t, withArgs(a, []string{"test", "a", "--max-warnings", tc.max}))
			block := normalize(checked.stdout, checked.tmp) + "\n"
			if got := normalize(tested.stdout, tested.tmp); !strings.HasPrefix(got, block) || !strings.Contains(block, warningHead) ||
				checked.code != tc.code || tested.code != tc.code {
				t.Errorf("check (exit %d):\n%s\ntest (exit %d, want %d):\n%s", checked.code, block, tested.code, tc.code, got)
			}
		})
	}
}

// warningAt is the head line of warning d at location at, in check's text form (API.md §4.4).
func warningAt(d *diag.Def, at string) string {
	return warningHead + string(d.Code) + "]  " + at
}

// withArgs is a with its args file replaced by args, one per line.
func withArgs(a *txtar.Archive, args []string) *txtar.Archive {
	out := &txtar.Archive{Comment: a.Comment}
	for _, f := range a.Files {
		if f.Name != argsFile && f.Name != wantFile {
			out.Files = append(out.Files, f)
		}
	}
	out.Files = append(out.Files, txtar.File{Name: argsFile, Data: []byte(strings.Join(args, "\n") + "\n")})
	return out
}
