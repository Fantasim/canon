package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/testkit/fixture"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// copyRenames is a temporary copy of the renames example with what it imports; expected/ stays behind.
func copyRenames(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := fixture.CopyRenames(dir, examplesDir); err != nil {
		t.Fatal(err)
	}
	return dir
}

// renameRun runs canon rename in dir.
func renameRun(dir string, args ...string) (code int, stdout, stderr string) {
	var out, errs bytes.Buffer
	argv := append([]string{"rename", "--project", dir}, args...)
	code = cli.Main(context.Background(), argv, cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
	return code, out.String(), errs.String()
}

// changedDiff is the unified diff of every file that differs between before and after, by path.
func changedDiff(t *testing.T, before, after map[string]string) string {
	t.Helper()
	var out strings.Builder
	names := make([]string, 0, len(after))
	for name := range after {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if before[name] == after[name] {
			continue
		}
		d, err := cli.UnifiedDiff(name, []byte(before[name]), []byte(after[name]))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&out, "[diff %s]\n%s", name, d)
	}
	return out.String()
}

// CLI.md §3.16, API.md E27, E29, E33, E34, E37: one rename per name kind on the renames example, undone by its printed undo.
func TestRenameExamples(t *testing.T) {
	golden.Run(t, "testdata/renames/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		dir := copyRenames(t)
		before := snapshot(t, dir)
		text, _ := archived(c.Archive, argsFile)
		code, out, errs := renameRun(dir, strings.Fields(text)...)
		after := snapshot(t, dir)
		got := fmt.Sprintf("[exit %d]\n[stdout]\n%s[stderr]\n%s%s", code, out, errs, changedDiff(t, before, after))
		if code == 0 {
			var o struct {
				Rename struct {
					Undo json.RawMessage `json:"undo"`
				} `json:"rename"`
			}
			first, _, _ := strings.Cut(out, "\n")
			if err := json.Unmarshal([]byte(first), &o); err != nil {
				t.Fatalf("rename object %q: %v", first, err)
			}
			if code, out, errs := editRun(t, dir, nil, string(o.Rename.Undo)); code != 0 || errs != "" {
				t.Fatalf("undo: exit %d, %s%s", code, out, errs)
			}
			if !equalFiles(before, snapshot(t, dir)) {
				t.Errorf("API.md E37: files differ after the undo %s", o.Rename.Undo)
			}
		} else if !equalFiles(before, after) {
			t.Error("a refused rename wrote files")
		}
		return []byte(jsonMs.ReplaceAllString(got, `"ms":0`))
	})
}
