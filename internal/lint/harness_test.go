package lint_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/lint"
	"golang.org/x/tools/txtar"
)

const (
	findingsFile = "findings.txt"
	selectFile   = "select" // the selectors, as for check; none selects every package
	applyFile    = "apply"  // present: the first fix is applied and the project linted again
	projectDir   = "/p"
	padOpen      = "{pad" // the opening of a pad line left in a fixture after the pad lines were expanded
)

// mapFS is a project under "/": fstest.MapFS with absolute names.
type mapFS fstest.MapFS

func rel(name string) string {
	if name == "/" {
		return "."
	}
	return strings.TrimPrefix(name, "/")
}

func (m mapFS) ReadFile(name string) ([]byte, error)       { return fstest.MapFS(m).ReadFile(rel(name)) }
func (m mapFS) Stat(name string) (fs.FileInfo, error)      { return fstest.MapFS(m).Stat(rel(name)) }
func (m mapFS) ReadDir(name string) ([]fs.DirEntry, error) { return fstest.MapFS(m).ReadDir(rel(name)) }

func file(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

// archiveFS is a case's files as a project under /p, its markers and expected output left out.
func archiveFS(t *testing.T, a *txtar.Archive) (fsys mapFS, pads int) {
	t.Helper()
	fsys = mapFS{}
	for _, f := range a.Files {
		switch f.Name {
		case findingsFile, selectFile, applyFile:
		default:
			text, n := padded(t, string(f.Data))
			fsys[path.Join("p", f.Name)] = file(text)
			pads += n
		}
	}
	return fsys, pads
}

// padRE is a line `{{pad N text}}`: N lines of text, each %d the index of its line.
var padRE = regexp.MustCompile(`(?m)^\{\{pad (\d+) ([^}]*)\}\}\n`)

// padded is src with its pad lines expanded, so a file of thousands of lines is written in a few, and
// the lines it expanded; a pad line left unexpanded or with a bad count fails the case.
func padded(t *testing.T, src string) (string, int) {
	t.Helper()
	expanded := 0
	out := padRE.ReplaceAllStringFunc(src, func(m string) string {
		sub := padRE.FindStringSubmatch(m)
		n, err := strconv.Atoi(sub[1])
		if err != nil {
			t.Fatal(err)
		}
		expanded++
		var b strings.Builder
		for i := range n {
			b.WriteString(strings.ReplaceAll(sub[2], "%d", strconv.Itoa(i)) + "\n")
		}
		return b.String()
	})
	if strings.Contains(out, padOpen) {
		t.Fatalf("a pad line is malformed and was not expanded (%s)", padOpen)
	}
	return out, expanded
}

// failOnErrors fails the case when its analysis holds an error finding: a padded fixture must be valid Canon.
func failOnErrors(t *testing.T, a *build.Analysis) {
	t.Helper()
	if n := a.Result().Summary.Errors; n > 0 {
		t.Fatalf("the padded fixture holds %d error findings, so it proves nothing", n)
	}
}

func marker(a *txtar.Archive, name string) (string, bool) {
	for _, f := range a.Files {
		if f.Name == name {
			return string(f.Data), true
		}
	}
	return "", false
}

// linted is a lint of the project fsys holds over selectors, with the project and analysis it read.
type linted struct {
	fsys      mapFS
	selectors []string
	p         *build.Project
	a         *build.Analysis
	res       *lint.Result
}

func analyzed(t *testing.T, fsys mapFS, selectors []string) *linted {
	t.Helper()
	return analyzedWith(t, fsys, selectors, build.Options{})
}

func analyzedWith(t *testing.T, fsys mapFS, selectors []string, opt build.Options) *linted {
	t.Helper()
	ctx := context.Background()
	p, err := build.Open(fsys, projectDir, opt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(ctx, selectors)
	if err != nil {
		t.Fatal(err)
	}
	res, err := lint.Run(ctx, a, edit.NewSnapshot(a))
	if err != nil {
		t.Fatal(err)
	}
	return &linted{fsys: fsys, selectors: selectors, p: p, a: a, res: res}
}

// render is a result as the golden holds it: the findings, then a line per fix.
func render(t *testing.T, a *build.Analysis, res *lint.Result) string {
	t.Helper()
	var b bytes.Buffer
	if err := diag.Render(&b, a.Files(), res.Findings, diag.RenderOptions{Summary: res.Summary, Golden: true}); err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Fixes {
		line, col := a.Files().Position(f.Span.File, f.Span.Start)
		req, err := json.Marshal(struct {
			Ops []edit.Operation `json:"ops"`
		}{f.Ops})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "fix %s %s:%d:%d %s\n", f.Code, a.Files().Path(f.Span.File), line, col, req)
	}
	return b.String()
}

// applyFirst applies the first fix of l to its files, as `canon edit` would, and lints again.
func applyFirst(t *testing.T, l *linted) string {
	t.Helper()
	if len(l.res.Fixes) == 0 {
		t.Fatal("apply: the case has no fix")
	}
	env := edit.Env{Project: l.p, Host: build.EditHost}
	plan, err := edit.Apply(context.Background(), env, edit.NewSnapshot(l.a), edit.Request{Ops: l.res.Fixes[0].Ops})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Changes {
		l.fsys[path.Join("p", c.Path)] = file(string(c.After))
	}
	again := analyzed(t, l.fsys, l.selectors)
	if n := again.a.Result().Summary.Errors; n > 0 {
		t.Fatalf("the project holds %d errors after the first fix, so the fix does not keep it valid", n)
	}
	return "after the first fix:\n" + render(t, again.a, again.res)
}

// lintCase is the golden of one case.
func lintCase(t *testing.T, a *txtar.Archive) string {
	t.Helper()
	sel, _ := marker(a, selectFile)
	selectors := strings.Fields(sel)
	fsys, pads := archiveFS(t, a)
	l := analyzed(t, fsys, selectors)
	if pads > 0 {
		failOnErrors(t, l.a)
	}
	out := render(t, l.a, l.res)
	if _, ok := marker(a, applyFile); ok {
		out += applyFirst(t, l)
	}
	return out
}
