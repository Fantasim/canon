package views_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/api/vm"
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
	"github.com/fantasim/canonlang/internal/views"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
	"github.com/fantasim/canonlang/internal/views/rules"
	"github.com/fantasim/canonlang/internal/views/shape"
)

const (
	lawDir      = "/law"
	examplesDir = "../../examples"
	studioPkg   = "studio"
	demoPkg     = "a"
	language    = "0.1"
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

// analyzed is a checked and evaluated project, the view model's inputs.
type analyzed struct {
	a      *build.Analysis
	studio string
	bags   check.Bags
	proj   *project.Project
	layout *project.Layout
	texts  map[string]*i18n.Result
	layers []string
}

// analyze runs phases 1 to 7 of the project in dir (EVALUATION.md 1): the view and translation
// checks of phase 2 included, as build wires them.
func analyze(t *testing.T, fsys project.FS, dir, studio string, opt build.Options) *analyzed {
	t.Helper()
	ctx := context.Background()
	p, err := build.Open(fsys, dir, opt)
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
	proj := loadProject(t, fsys, dir)
	layout, _ := project.NewLayout(proj, dir, opt.Roots, diag.NewBag(nil, ""))
	prog, emits := a.Program(), emitsView(a.Program())
	rules.Check(ctx, prog, bags, studio, emits)
	units, _ := a.Force(eval.Root{Pkg: studio, Name: syntax.StudioUnits})
	rules.CheckUnits(ctx, prog, bags, studio, units)
	texts := i18n.Check(prog, proj, bags, emits)
	return &analyzed{a: a, studio: studio, bags: bags, proj: proj, layout: layout, texts: texts, layers: opt.Layers}
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

// emitsView are the packages declaring `emit view`, as build finds them (VIEWMODEL.md N4).
func emitsView(prog *check.Program) map[string]bool {
	out := map[string]bool{}
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				e, ok := d.(*syntax.EmitDecl)
				out[p.Path] = out[p.Path] || ok && e.Target != nil && e.Target.Name == check.TargetView
			}
		}
	}
	return out
}

// demo analyzes one package `a` (and a studio package when studioSrc is not empty).
func demo(t *testing.T, src, studioSrc string) *analyzed {
	t.Helper()
	files := map[string]string{"a/a.canon": src}
	if studioSrc != "" {
		files["studio/studio.canon"] = studioSrc
	}
	return tree(t, "", files, build.Options{})
}

// tree analyzes a project of files under /law (a studio package when files hold one), with the
// project keys head adds; any error fails it.
func tree(t *testing.T, head string, files map[string]string, opt build.Options) *analyzed {
	t.Helper()
	x := broken(t, head, files, opt)
	//canon:unordered any error fails the demo, whichever package holds it
	for _, b := range x.bags {
		for _, f := range b.Findings() {
			if f.Severity == diag.Error {
				t.Fatalf("demo: %s %s at byte %d", f.Code, f.Message, f.Span.Start)
			}
		}
	}
	return x
}

// broken analyzes a project as tree does, errors included (VIEWMODEL.md J4).
func broken(t *testing.T, head string, files map[string]string, opt build.Options) *analyzed {
	t.Helper()
	proj := "project demo {\n  canon: \"0.1\"\n" + head
	fsys := mapFS{}
	studio := ""
	//canon:unordered a map copied into a map
	for name, src := range files {
		fsys["law/"+name] = &fstest.MapFile{Data: []byte(src)}
		if strings.HasPrefix(name, studioPkg+"/") {
			studio = studioPkg
		}
	}
	if studio != "" {
		proj += "  studio: studio\n"
	}
	fsys["law/project.canon"] = &fstest.MapFile{Data: []byte(proj + "}\n")}
	return analyze(t, fsys, lawDir, studio, opt)
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
	return analyze(t, project.OS(), filepath.ToSlash(dir), studioPkg, build.Options{Roots: roots})
}

// model is pkg's view model.
func (p *analyzed) model(t *testing.T, pkg string) *vm.ViewModel {
	t.Helper()
	return p.modelWith(t, pkg, standIn{info: p.a.Program().Info})
}

// modelWith is pkg's view model, its view expressions evaluated by ev.
func (p *analyzed) modelWith(t *testing.T, pkg string, ev render.Evaluator) *vm.ViewModel {
	t.Helper()
	var found []diag.Finding
	if b := p.bags[pkg]; b != nil {
		found = b.Findings()
	}
	m, err := views.Build(context.Background(), views.Input{
		Program: p.a.Program(), Package: pkg, Language: language, Studio: p.studio, Languages: p.proj.Languages,
		Force: p.a.Force, Fold: eval.NewFolder(p.bags, eval.Options{}), I18N: p.texts, Layout: p.layout, Layers: p.layers,
		Findings: found, Files: p.a.Files(), Errors: p.allFindings(), Eval: ev,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// resolver resolves the controls of the project, entries counted in this build.
func (p *analyzed) resolver() *control.Resolver {
	colls := encode.NewColls(func(pkg, name string) (value.Value, bool) {
		return p.a.Force(eval.Root{Pkg: pkg, Name: name})
	})
	fold := control.FoldWith(context.Background(), p.a.Program(), eval.NewFolder(p.bags, eval.Options{}))
	return control.NewResolver(control.NewIndex(p.a.Program(), p.studio, nil), control.Env{
		Counts: colls.Counts, Fold: fold, Assets: encode.NewAssets(p.a.Program(), p.layout),
	})
}

// named is the type pkg declares under name, aliases expanded.
func (p *analyzed) named(t *testing.T, pkg, name string) types.Type {
	t.Helper()
	for _, cp := range p.a.Program().Packages {
		for _, o := range cp.Decls {
			if cp.Path == pkg && o.Name() == name && o.Kind() == check.ObjTypeName {
				return shape.Unalias(o.Type())
			}
		}
	}
	t.Fatalf("%s has no type %s", pkg, name)
	return nil
}

// letType is the declared type of the top-level let name of pkg.
func (p *analyzed) letType(t *testing.T, pkg, name string) types.Type {
	t.Helper()
	for _, cp := range p.a.Program().Packages {
		for _, o := range cp.Decls {
			if cp.Path == pkg && o.Name() == name && o.Kind() == check.ObjLet {
				return o.Type()
			}
		}
	}
	t.Fatalf("%s has no let %s", pkg, name)
	return nil
}

// field is the field name of the record or case rec.
func field(t *testing.T, rec types.Type, name string) *types.Field {
	t.Helper()
	for _, f := range encode.FieldsOf(rec) {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("%s has no field %s", rec, name)
	return nil
}

// canonical is v as JSON, re-read so that two encodings of one value compare equal.
func canonical(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, b)
}

func decode(t *testing.T, b []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// text is v as compact JSON, for a failure message.
func text(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// allFindings are every package's findings, packages in path order (VIEWMODEL.md J4).
func (p *analyzed) allFindings() []diag.Finding {
	var out []diag.Finding
	for _, name := range slices.Sorted(maps.Keys(p.bags)) {
		out = append(out, p.bags[name].Findings()...)
	}
	return out
}
