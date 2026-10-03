package tsgen_test

import (
	"context"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	tsgen "github.com/fantasim/canonlang/internal/gen/ts"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	canonExt    = ".canon"
	generatedTS = "ts.out"
)

// world is the sources of a case after parsing, checking and stage E, with the evaluator's vectors filled in: what the real front end gives the generator.
type world struct {
	fs    *source.FileSet
	parse *diag.Bag
	bags  check.Bags
	proj  *project.Project
	pkgs  []*ir.Package
}

// buildWorld runs the pipeline on the .canon files of files, under the examples' project (its roots place every emit).
func buildWorld(t *testing.T, files []txtar.File) *world {
	t.Helper()
	ctx := context.Background()
	w := &world{fs: &source.FileSet{}, bags: check.Bags{}}
	w.parse = diag.NewBag(w.fs, "")
	w.proj = loadProject(t, w.fs, w.parse)
	var parsed []*syntax.File
	for _, f := range files {
		if path.Ext(f.Name) != canonExt {
			continue
		}
		src, err := w.fs.Add(f.Name, "/"+f.Name, f.Data)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, syntax.Parse(src, syntax.FileSource, w.parse))
	}
	prog := check.Check(ctx, w.proj, parsed, w.bags, eval.NewFolder(w.bags, eval.Options{}))
	ev := eval.New(prog, served{}, w.bags, eval.Options{})
	w.pkgs = ir.Build(ctx, ir.Input{Program: prog, Project: w.proj, Bags: w.bags, Host: irHost{ev}, Fold: eval.NewFolder(w.bags, eval.Options{})})
	if err := conform.Fill(ctx, prog, w.pkgs, conformer{prog: prog, ev: ev}, w.bags); err != nil {
		t.Fatal(err)
	}
	return w
}

// errors are the findings of severity error, rendered; a case that must build has none.
func (w *world) findings(t *testing.T) (all []diag.Finding, errors int) {
	t.Helper()
	all = w.parse.Findings()
	errors = w.parse.Summary().Errors
	for _, name := range slices.Sorted(maps.Keys(w.bags)) {
		all = append(all, w.bags[name].Findings()...)
		errors += w.bags[name].Summary().Errors
	}
	return all, errors
}

// output is a generated file with its project-relative path.
type output struct {
	Path    string
	Content []byte
}

// generate runs the TypeScript generator on every ts emit of every package, each on the package as its copy sees it, twice (CODEGEN.md §2.7: equal outputs); files come in path order.
func (w *world) generate(t *testing.T) []output {
	t.Helper()
	var out []output
	for _, p := range w.pkgs {
		for _, e := range p.Emits {
			if e.Target == ir.TargetTS {
				out = append(out, w.generateEmit(t, p, e)...)
			}
		}
	}
	slices.SortFunc(out, func(a, b output) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// generateEmit runs the generator twice on one emit and returns its files at their project-relative paths.
func (w *world) generateEmit(t *testing.T, p *ir.Package, e *ir.Emit) []output {
	t.Helper()
	cp := ir.CopyOf(w.proj, p, e)
	files, err := tsgen.Generate(cp, e)
	if err != nil {
		t.Fatalf("%s: %v", p.Name, err)
	}
	again, err := tsgen.Generate(cp, e)
	if err != nil || len(again) != len(files) {
		t.Fatalf("%s: second run: %v", p.Name, err)
	}
	out := make([]output, len(files))
	for i, f := range files {
		if string(f.Content) != string(again[i].Content) {
			t.Errorf("%s: %s differs between two runs", p.Name, f.Path)
		}
		out[i] = output{Path: path.Join(e.Dir, f.Path), Content: f.Content}
	}
	return out
}

func loadProject(t *testing.T, fs *source.FileSet, bag *diag.Bag) *project.Project {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examplesDir, projectFile))
	if err != nil {
		t.Fatal(err)
	}
	src, err := fs.Add(projectFile, "/"+projectFile, data)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := project.Load(src, bag)
	if err != nil {
		t.Fatal(err)
	}
	return proj
}

// generateJSON runs the JSON generator on every json emit: the data files the data-mode decoders read (WIRE.md §8), by file name.
func (w *world) generateJSON(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, p := range w.pkgs {
		for _, e := range p.Emits {
			if e.Target != ir.TargetJSON {
				continue
			}
			files, err := jsongen.Generate(ir.CopyOf(w.proj, p, e), e)
			if err != nil {
				t.Fatalf("%s: %v", p.Name, err)
			}
			for _, f := range files {
				out[path.Base(f.Path)] = f.Content
			}
		}
	}
	return out
}
