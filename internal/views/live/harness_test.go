package live_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/live"
	"github.com/fantasim/canonlang/internal/views/render"
	"github.com/fantasim/canonlang/internal/views/rules"
)

const (
	lawDir      = "/law"
	examplesDir = "../../../examples"
	studioPkg   = "studio"
)

// mapFS is an in-memory project tree whose paths are absolute.
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

// analyzed is a checked and evaluated project, what Evaluate reads.
type analyzed struct {
	a      *build.Analysis
	studio string
	texts  map[string]*i18n.Result
	langs  []string
}

// setup is a project to analyze: its tree and directory, studio package and build options; an
// error of any package fails the test when strict.
type setup struct {
	fsys   project.FS
	dir    string
	studio string
	opt    build.Options
	strict bool
}

// analyze runs phases 1 to 7 of the project, with the view and translation checks of phase 2,
// as build wires them.
func analyze(t *testing.T, in setup) *analyzed {
	t.Helper()
	ctx := context.Background()
	fsys, dir, studio, strict := in.fsys, in.dir, in.studio, in.strict
	p, err := build.Open(fsys, dir, in.opt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{}
	for _, cp := range a.Program().Packages {
		if b := a.Bag(cp.Path); b != nil {
			bags[cp.Path] = b
		}
	}
	prog := a.Program()
	rules.Check(ctx, prog, bags, studio, nil)
	texts := i18n.Check(prog, loadProject(t, fsys, dir), bags, nil)
	for _, cp := range prog.Packages {
		if bags[cp.Path] == nil || !strict {
			continue
		}
		for _, f := range bags[cp.Path].Findings() {
			if f.Severity == diag.Error {
				t.Fatalf("%s: %s %s at byte %d", cp.Path, f.Code, f.Message, f.Span.Start)
			}
		}
	}
	return &analyzed{a: a, studio: studio, texts: texts, langs: loadProject(t, fsys, dir).Languages}
}

// loadProject is the project.canon of dir.
func loadProject(t *testing.T, fsys project.FS, dir string) *project.Project {
	t.Helper()
	name := dir + "/" + project.FileName
	data, err := fsys.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	set := &source.FileSet{}
	src, err := set.Add(project.FileName, name, data)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := project.Load(src, diag.NewBag(set, ""))
	if err != nil {
		t.Fatal(err)
	}
	return proj
}

// demo analyzes a project of files under /law, its project.canon given head's keys.
func demo(t *testing.T, head string, files map[string]string) *analyzed {
	t.Helper()
	fsys := mapFS{"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n" + head + "}\n")}}
	//canon:unordered a map copied into a map
	for name, src := range files {
		fsys["law/"+name] = &fstest.MapFile{Data: []byte(src)}
	}
	return analyze(t, setup{fsys: fsys, dir: lawDir, strict: true})
}

// examples analyzes the examples project, every root outside it redirected.
func examples(t *testing.T) *analyzed {
	t.Helper()
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(out, name))
	}
	return analyze(t, setup{fsys: project.OS(), dir: filepath.ToSlash(dir), studio: studioPkg, opt: build.Options{Roots: roots}})
}

// input is what Evaluate reads of p, its view expressions evaluated by standIn.
func (p *analyzed) input() live.Input {
	force := func(pkg, name string) (value.Value, bool) { return p.a.Force(eval.Root{Pkg: pkg, Name: name}) }
	ev := standIn{info: p.a.Program().Info}
	return live.Input{Program: p.a.Program(), Studio: p.studio, I18N: p.texts, Force: force, Eval: ev, Lines: lines{}, Languages: p.langs}
}

// let is the settled value of pkg's let name.
func (p *analyzed) let(t *testing.T, pkg, name string) value.Value {
	t.Helper()
	v, ok := p.a.Force(eval.Root{Pkg: pkg, Name: name})
	if !ok {
		t.Fatalf("%s:%s has no value", pkg, name)
	}
	return v
}

// entry is the table entry or keyed-list element key of the collection v.
func entry(t *testing.T, v value.Value, key string) *value.Record {
	t.Helper()
	var es []*value.Record
	switch x := v.(type) {
	case *value.Table:
		es = x.Entries
	case *value.List:
		for _, e := range x.Elems {
			es = append(es, e.(*value.Record))
		}
	}
	for _, e := range es {
		if e.Ident != nil && e.Ident.Key.Text() == key {
			return e
		}
	}
	t.Fatalf("no entry %s", key)
	return nil
}

// standIn stands for the evaluator, which Analysis does not expose: magic names, `self`, fields,
// a ref's `.id`, integer `>` and `*`. It fails where any evaluator does (a magic name without a
// value, G12; a read of `none`), and on anything else, which the tests do not use.
type standIn struct{ info *check.Info }

func (s standIn) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	switch x := e.(type) {
	case *syntax.ParenExpr:
		return s.Eval(ctx, x.X, self, m)
	case *syntax.SelfExpr:
		return self, self != nil
	case *syntax.SelectorExpr:
		v, ok := s.Eval(ctx, x.X, self, m)
		if x.Name == nil {
			return nil, false
		}
		if r, isRef := v.(*value.Ref); ok && isRef && x.Name.Name == "id" {
			return &value.Str{V: r.Key.Text(), T: types.StringType}, true
		}
		if !ok {
			return nil, false
		}
		return fieldOf(v, x.Name.Name)
	case *syntax.BinaryExpr:
		return s.binary(ctx, x, self, m)
	case *syntax.IntLit:
		if x.Value == nil {
			return nil, false
		}
		return &value.Int{V: x.Value.Int64(), T: types.IntType}, true
	case *syntax.IdentExpr:
		switch o := s.info.Uses[x]; {
		case o == nil:
		case o.Kind() == check.ObjField:
			return fieldOf(self, x.Name)
		case o.Kind() == check.ObjLocal:
			v := map[string]value.Value{"id": m.ID, "key": m.Key, "index": m.Index}[x.Name]
			return v, v != nil
		}
	}
	return nil, false
}

// binary is `a > b` or `a * b` on integers.
func (s standIn) binary(ctx context.Context, x *syntax.BinaryExpr, self value.Value, m render.Magic) (value.Value, bool) {
	a, ok := s.Eval(ctx, x.X, self, m)
	b, ok2 := s.Eval(ctx, x.Y, self, m)
	ai, isInt := a.(*value.Int)
	bi, isInt2 := b.(*value.Int)
	if !ok || !ok2 || !isInt || !isInt2 {
		return nil, false
	}
	switch x.Op {
	case syntax.TokGt:
		return &value.Bool{V: ai.V > bi.V}, true
	case syntax.TokStar:
		return &value.Int{V: ai.V * bi.V, T: types.IntType}, true
	}
	return nil, false
}

// fieldOf is the field name of the record self; false when self is no record (`none`).
func fieldOf(self value.Value, name string) (value.Value, bool) {
	r, ok := self.(*value.Record)
	if !ok {
		return nil, false
	}
	for i, f := range encode.FieldsOf(r.T) {
		if f.Name == name && i < len(r.Fields) && r.Fields[i] != nil {
			return r.Fields[i], true
		}
	}
	return nil, false
}

// lines renders a show line's source template through its Eval, each value in its canonical
// text, and a method as `<name>()` but the method `broken`, which fails.
type lines struct{}

func (l lines) Show(line live.Line, self *value.Record, _ string) (string, bool) {
	sl, ok := line.Template.(*syntax.StringLit)
	if !ok {
		return "", false
	}
	var b strings.Builder
	for _, p := range sl.Parts {
		if p.Interp == nil {
			b.WriteString(p.Text)
			continue
		}
		v, ok := line.Eval.Eval(context.Background(), p.Interp.X, self, line.Magic)
		if !ok {
			return "", false
		}
		b.WriteString(v.CanonText())
	}
	return b.String(), true
}

func (l lines) Method(_ *value.Record, name, _ string) (string, bool) {
	return name + "()", name != "broken"
}

// jsonOf is v as indented-free JSON, for comparisons.
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
