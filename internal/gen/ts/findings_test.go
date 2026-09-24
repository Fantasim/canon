package tsgen_test

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	findingsFile = "findings.txt"
	projectFile  = "project.canon"
	examplesDir  = "../../../examples"
)

// CONFORMANCE.md §4, §7: each case's translated fns get their vectors from the real evaluator; the TS column pins the code.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		st := stageE(t, c)
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(st, code) {
			t.Errorf("%s does not produce %s:\n%s", c.Path, code, st)
		}
		return []byte(st)
	}, golden.Expected(findingsFile))
}

// stageE checks a case's sources under the examples' project, runs stage A and E, fills the
// vectors through eval, and prints each translated fn's vectors, then the findings.
func stageE(t *testing.T, c golden.Case) string {
	t.Helper()
	ctx := context.Background()
	fs := &source.FileSet{}
	parse := diag.NewBag(fs, "")
	proj := loadProject(t, fs, parse)
	var files []*syntax.File
	for _, f := range c.Archive.Files {
		if f.Name == findingsFile {
			continue
		}
		src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, syntax.Parse(src, syntax.FileSource, parse))
	}
	bags := check.Bags{}
	prog := check.Check(ctx, proj, files, bags, eval.NewFolder(bags, eval.Options{}))
	ev := eval.New(prog, served{}, bags, eval.Options{})
	pkgs := ir.Build(ctx, ir.Input{Program: prog, Project: proj, Bags: bags, Host: irHost{ev}, Fold: eval.NewFolder(bags, eval.Options{})})
	if err := conform.Fill(ctx, prog, pkgs, conformer{prog: prog, ev: ev}, bags); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	for _, p := range pkgs {
		for _, fn := range p.Fns {
			printVectors(&out, fn)
		}
	}
	all, sum := parse.Findings(), parse.Summary()
	sum.Packages = 0
	for _, name := range slices.Sorted(maps.Keys(bags)) {
		all = append(all, bags[name].Findings()...)
		sum = sum.Merge(bags[name].Summary())
	}
	if err := diag.Render(&out, fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return out.String()
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

// printVectors writes one line per vector: its arguments, then the Go/C++ and TS expectations.
func printVectors(w *bytes.Buffer, fn *ir.ExportFn) {
	for i, v := range fn.Vectors {
		args := make([]string, len(v.Args))
		for j, a := range v.Args {
			args[j] = a.CanonText()
		}
		fmt.Fprintf(w, "%s vector %d (%s): go %s, ts %s\n", fn.Name, i+1, strings.Join(args, ", "), expected(v.Want, v.Code), expected(v.TSWant, v.TSCode))
	}
}

func expected(v value.Value, code diag.Code) string {
	if code != "" {
		return string(code)
	}
	if v == nil {
		return "-"
	}
	return v.CanonText()
}

// served is a host that serves no file and finds every value valid.
type served struct{}

func (served) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	return nil, false
}

func (served) Verify(context.Context, eval.Root, value.Value) bool { return true }

// irHost is stage A's evaluator as stage E's host.
type irHost struct {
	ev *eval.Evaluator
}

func (h irHost) Value(ctx context.Context, pkg, name string) (value.Value, bool) {
	return h.ev.Force(ctx, eval.Root{Pkg: pkg, Name: name})
}

func (h irHost) Call(ctx context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool) {
	return h.ev.Call(ctx, fn, recv, args)
}

// conformer is conform.Evaluator over eval, as build adapts it: tests on a fresh evaluator
// whose findings are thrown away, vectors on stage A's.
type conformer struct {
	prog *check.Program
	ev   *eval.Evaluator
}

// limits maps eval's limits onto conform's.
var limits = [...]conform.Limit{eval.NoLimit: conform.NoLimit, eval.StepLimit: conform.StepLimit, eval.DepthLimit: conform.DepthLimit}

func (c conformer) TestCalls(ctx context.Context, pkg string, fns []check.Object) []conform.Call {
	calls, err := eval.New(c.prog, served{}, check.Bags{}, eval.Options{}).TestCalls(ctx, pkg, fns, nil)
	if err != nil {
		return nil
	}
	out := make([]conform.Call, len(calls))
	for i, x := range calls {
		out[i] = conform.Call{Fn: x.Fn, Recv: x.Recv, Args: x.Args}
	}
	return out
}

func (c conformer) Evaluate(ctx context.Context, x conform.Call, m conform.Mode) conform.Outcome {
	o := c.ev.Vector(ctx, eval.Call{Fn: x.Fn, Recv: x.Recv, Args: x.Args}, eval.VectorMode{Steps: m.Steps, TS: m.TS})
	return conform.Outcome{Value: o.Value, Code: o.Code, Exceeded: limits[o.Exceeded]}
}
