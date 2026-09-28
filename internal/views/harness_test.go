package views_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
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
}

// analyze runs phases 1 to 7 of the project in dir (EVALUATION.md §1).
func analyze(t *testing.T, fsys project.FS, dir, studio string, opt build.Options) *analyzed {
	t.Helper()
	p, err := build.Open(fsys, dir, opt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{}
	for _, cp := range a.Program().Packages {
		if b := a.Bag(cp.Path); b != nil {
			bags[cp.Path] = b
		}
	}
	return &analyzed{a: a, studio: studio, bags: bags}
}

// demo analyzes one package `a` (and a studio package when studioSrc is not empty).
func demo(t *testing.T, src, studioSrc string) *analyzed {
	t.Helper()
	proj := "project demo {\n  canon: \"0.1\"\n"
	fsys := mapFS{"law/a/a.canon": &fstest.MapFile{Data: []byte(src)}}
	studio := ""
	if studioSrc != "" {
		proj += "  studio: studio\n"
		fsys["law/studio/studio.canon"] = &fstest.MapFile{Data: []byte(studioSrc)}
		studio = studioPkg
	}
	fsys["law/project.canon"] = &fstest.MapFile{Data: []byte(proj + "}\n")}
	x := analyze(t, fsys, lawDir, studio, build.Options{})
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
	m, err := views.Build(context.Background(), views.Input{
		Program: p.a.Program(), Package: pkg, Language: language, Studio: p.studio,
		Force: p.a.Force, Fold: eval.NewFolder(p.bags, eval.Options{}),
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
	return control.NewResolver(control.NewIndex(p.a.Program(), p.studio), control.Env{Counts: colls.Counts, Fold: fold})
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
