package eval_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"golang.org/x/tools/txtar"
)

const (
	examplesDir = "../../examples"
	projectFile = "project.canon"
	canonExt    = ".canon"
)

// exampleRoots are the roots examples/project.canon declares.
var exampleRoots = []string{"client", "features", "generated", "parity", "pipeline_go", "resource", "services", "source", "sovcommon", "web"}

// program is one parsed program: its files, and the parse findings.
type program struct {
	fs    *source.FileSet
	files []*syntax.File
	parse *diag.Bag
}

// parseFiles parses named sources, in the order given.
func parseFiles(t testing.TB, names []string, data [][]byte) *program {
	t.Helper()
	p := &program{fs: &source.FileSet{}}
	p.parse = diag.NewBag(p.fs, "")
	for i, name := range names {
		src, err := p.fs.Add(name, "/"+name, data[i])
		if err != nil {
			t.Fatal(err)
		}
		p.files = append(p.files, syntax.Parse(src, syntax.FileSource, p.parse))
	}
	return p
}

// fromArchive parses the .canon files of a txtar archive.
func fromArchive(t testing.TB, a *txtar.Archive) *program {
	t.Helper()
	var names []string
	var data [][]byte
	for _, f := range a.Files {
		if path.Ext(f.Name) == canonExt {
			names, data = append(names, f.Name), append(data, f.Data)
		}
	}
	return parseFiles(t, names, data)
}

// fromExamples parses every .canon file of examples/ under one of dirs, in path order.
func fromExamples(t testing.TB, dirs ...string) *program {
	t.Helper()
	var names []string
	var data [][]byte
	err := filepath.WalkDir(examplesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != canonExt || d.Name() == projectFile {
			return err
		}
		rel, err := filepath.Rel(examplesDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if len(dirs) > 0 && !slices.ContainsFunc(dirs, func(d string) bool { return strings.HasPrefix(rel, d+"/") }) {
			return nil
		}
		b, err := os.ReadFile(p)
		names, data = append(names, rel), append(data, b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return parseFiles(t, names, data)
}

func exampleProject() *project.Project {
	p := project.New("sovereign", project.Version{Major: 0, Minor: 1})
	for _, r := range exampleRoots {
		p.Roots = append(p.Roots, project.Root{Name: r, Path: r})
	}
	return p
}

// build is one run of phases 2 to 6 over a program (EVALUATION.md §1), with verify and rules.
type build struct {
	prog    *program
	bags    check.Bags
	checked *check.Program
	ev      *eval.Evaluator
	values  map[eval.Root]value.Value
	order   []eval.Root
}

// host is the fixture eval.Host: no file is loaded, and stage B is verify's, its Result mapped
// onto the frozen bool as a build adapter will (DECISIONS: Host.Verify adapter).
type host struct {
	verifier *verify.Verifier
	ev       *eval.Evaluator
}

func (h *host) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	return nil, false
}

func (h *host) Verify(ctx context.Context, root eval.Root, v value.Value) bool {
	res, err := h.verifier.Check(ctx, root, v)
	if err != nil {
		return false
	}
	if res.Poisoned {
		h.ev.Poison(root)
	}
	for _, u := range res.Unbound {
		h.ev.ReportUnbound(root, u.Ref, u.Path)
	}
	return res.Valid
}

// checks adapts the evaluator to rules.Evaluator: eval.CheckRun to rules.Run.
type checks struct {
	*eval.Evaluator
}

func (c checks) Run(ctx context.Context, d *syntax.CheckDecl, self value.Value) rules.Run {
	x := c.Evaluator.Run(ctx, d, self)
	out := rules.Run{Aborted: x.Aborted, Failed: x.Failed, Message: x.Message}
	for _, r := range x.Reports {
		out.Reports = append(out.Reports, rules.Report{Warn: r.Warn, At: r.At, Message: r.Message})
	}
	return out
}

// runBuild checks the program and evaluates the selected packages (all when none is named):
// stage A in forced-set order, B, C, then D.
func runBuild(t testing.TB, p *program, opt eval.Options, selected ...string) *build {
	t.Helper()
	ctx := context.Background()
	b := &build{prog: p, bags: check.Bags{}, values: map[eval.Root]value.Value{}}
	fold := eval.NewFolder(b.bags, opt)
	b.checked = check.Check(ctx, exampleProject(), p.files, b.bags, fold)
	h := &host{}
	b.ev = eval.New(b.checked, h, b.bags, opt)
	h.ev = b.ev
	h.verifier = verify.New(b.ev, b.checked, b.bags, nil)
	pkgs := b.selected(selected)
	for _, pkg := range pkgs {
		for _, obj := range pkg.Decls {
			if obj.Kind() == check.ObjConst || obj.Kind() == check.ObjLet {
				root := eval.Root{Pkg: pkg.Path, Name: obj.Name()}
				b.order = append(b.order, root)
				b.ev.Force(ctx, root)
			}
		}
	}
	b.ev.BeginVerification(ctx)
	b.stagesCD(ctx, pkgs)
	if err := errors.Join(b.ev.Err(), eval.FoldErr(fold)); err != nil {
		t.Errorf("internal error: %v", err)
	}
	return b
}

func (b *build) selected(names []string) []*check.Package {
	var out []*check.Package
	for _, pkg := range b.checked.Packages {
		if len(names) == 0 || slices.Contains(names, pkg.Path) {
			out = append(out, pkg)
		}
	}
	return out
}

// stagesCD runs the instance checks of every value, then the package checks.
func (b *build) stagesCD(ctx context.Context, pkgs []*check.Package) {
	runner := rules.New(checks{b.ev}, b.checked, b.bags)
	for _, root := range b.order {
		if v, ok := b.ev.Force(ctx, root); ok {
			b.values[root] = v
			_ = runner.Instances(ctx, root, v)
		}
	}
	for _, pkg := range pkgs {
		_ = runner.Names(pkg)
		_ = runner.Package(ctx, pkg)
	}
}

// findings renders the parse findings and every bag's, packages by path.
func (b *build) findings(t testing.TB) string {
	t.Helper()
	all := b.prog.parse.Findings()
	sum := b.prog.parse.Summary()
	sum.Packages = 0
	names := make([]string, 0, len(b.bags))
	for name := range b.bags {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		all = append(all, b.bags[name].Findings()...)
		sum = sum.Merge(b.bags[name].Summary())
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, b.prog.fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// dump prints every forced value in forced-set order, poisoned ones marked.
func (b *build) dump() string {
	var sb strings.Builder
	for _, root := range b.order {
		sb.WriteString(root.Pkg + "." + root.Name)
		if v, ok := b.values[root]; ok {
			sb.WriteString(" = " + v.CanonText())
		} else {
			sb.WriteString(" poisoned")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
