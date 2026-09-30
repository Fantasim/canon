package workspace_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/workspace"
)

// twinSource loads one directory of JSON rows three ways: by its name, through a link to it,
// and one row through that link, so a row is read under two names.
const twinSource = `/// T.
package t

/// A row.
record Row {
  /// Id.
  id: String
  /// N.
  n: Int
}

/// Rows.
let rows: [Row] keyed by id = load.dir("rows/*.json")

/// Rows through a link.
let linked: [Row] keyed by id = load.dir("link/*.json")

/// One row through the link.
let one: Row = load("link/r1.json")
`

// twinStep changes the twin on disk or in the project.
type twinStep struct {
	name string
	do   func(p *workspace.Project) error
	over bool // the step leaves an overlay: the cold analysis reads the snapshot's files
}

// API.md S1, S3, S5, §3.4, IMPLEMENTATION-PLAN §7.6 (P18): a twin taken by sum stays cold, whatever changes.
func TestSumsTwinEqualsCold(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	put := func(name, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put(project.FileName, "project acme {\n  canon: \"0.1\"\n}\n")
	put("t/t.canon", twinSource)
	for _, r := range []string{"r1", "r2", "r3"} {
		put("t/rows/"+r+".json", fmt.Sprintf("{\"id\": %q, \"n\": 1}\n", r))
	}
	if os.Symlink("rows", filepath.Join(dir, "t", "link")) != nil {
		t.Skip("no symbolic links here")
	}
	root := filepath.ToSlash(dir)
	b, err := build.Open(build.OS(), root, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	disk := func(data string) func(*workspace.Project) error {
		return func(*workspace.Project) error { put("t/rows/r2.json", data); return nil }
	}
	prev := ""
	for _, st := range []twinStep{
		{"first", func(*workspace.Project) error { return nil }, false},
		{"r2 changed on disk", disk("{\"id\": \"r2\", \"n\": 22}\n"), false},
		{"r1 changed, read through the link", func(*workspace.Project) error {
			put("t/rows/r1.json", "{\"id\": \"r1\", \"n\": 111}\n")
			return nil
		}, false},
		{"overlay on r3", func(p *workspace.Project) error {
			return p.SetOverlay("t/rows/r3.json", []byte("{\"id\": \"r3\", \"n\": 3333}\n"))
		}, true},
		{"r2 changed under the overlay", disk("{\"id\": \"r2\", \"n\": 2}\n"), true},
		{"overlay cleared", func(p *workspace.Project) error { return p.ClearOverlay("t/rows/r3.json") }, false},
	} {
		if err := st.do(p); err != nil {
			t.Fatal(err)
		}
		s := read(t, p)
		a := analyze(t, s)
		warm := twinDump(t, a)
		if a.Result().Summary.Errors != 0 || warm == prev {
			t.Errorf("%s: %d errors, the analysis unchanged: %t", st.name, a.Result().Summary.Errors, warm == prev)
		}
		prev = warm
		coldFS := build.OS()
		if st.over {
			coldFS = nil
		}
		if cold := twinDump(t, coldAnalysis(t, s, coldFS, root)); warm != cold {
			t.Errorf("%s: warm\n%s\ncold\n%s", st.name, warm, cold)
		}
	}
}

// coldAnalysis is a cold analysis of the project in root over fsys, or with none over s's files.
func coldAnalysis(t *testing.T, s *workspace.Snapshot, fsys build.WriteFS, root string) *build.Analysis {
	t.Helper()
	var over project.FS = fsys
	if fsys == nil {
		over = s.Build().FS()
	}
	b, err := build.Open(over, root, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := b.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// twinDump is what an analysis shows: findings, revision, manifest, each let's value and each
// package's read set.
func twinDump(t *testing.T, a *build.Analysis) string {
	t.Helper()
	res := a.Result()
	var sb strings.Builder
	opt := diag.RenderOptions{Format: diag.FormatText, Summary: res.Summary, Golden: true}
	if err := diag.Render(&sb, res.Files, res.List, opt); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&sb, "revision %s\n", res.Revision)
	sb.Write(a.Manifest())
	for _, cp := range a.Program().Packages {
		for _, obj := range cp.Decls {
			if obj.Kind() != check.ObjLet {
				continue
			}
			v, ok := a.Force(eval.Root{Pkg: cp.Path, Name: obj.Name()})
			text := "-"
			if v != nil {
				text = v.CanonText()
			}
			fmt.Fprintf(&sb, "%s.%s ok=%t %s\n", cp.Path, obj.Name(), ok, text)
		}
		for _, r := range a.Reads(cp.Path) {
			fmt.Fprintf(&sb, "read %s dir=%t sources=%t link=%t\n", r.Display, r.Dir, r.Sources, r.Link)
		}
	}
	return sb.String()
}
