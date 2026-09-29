package views

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
	"github.com/fantasim/canonlang/internal/views/shape"
	"github.com/fantasim/canonlang/internal/views/table"
	"github.com/fantasim/canonlang/internal/views/typedef"
)

// Input is what a package's view model is built from (VIEWMODEL.md 12).
type Input struct {
	Program   *check.Program
	Package   string                              // the package whose model is built
	Language  string                              // project.canon's `canon` version, "0.1"
	Studio    string                              // project.studio's package, "" for none
	Languages []string                            // project.languages, the source language first
	Force     func(eval.Root) (value.Value, bool) // a value the model reads, evaluated on demand if need be; nil: none
	Fold      check.Folder                        // folds constant field defaults; nil writes none
	I18N      map[string]*i18n.Result             // every package's catalogue and translations (i18n.Check)
	Layout    *project.Layout                     // places loads and asset roots; nil: none written
	Layers    []string                            // the active layers, in application order
	Findings  []diag.Finding                      // the package's findings of phases 1-7, F2 order (J15)
	Files     diag.Files                          // locates Findings
	Eval      render.Evaluator                    // evaluates view expressions; nil: nothing rendered
	CheckRun  CheckRun                            // the check run behind a finding, for its translated messages (J15); nil: none
	Drivers   func() (*World, error)              // the program typeFunction drivers are read over (12.3, J5), asked on first need; nil: Program and Force
}

// World is a program wider than Input.Program, every package that may pass a collection to the
// package's type functions, and its values, evaluated on demand (VIEWMODEL.md 12.3, J5).
type World struct {
	Program *check.Program
	Force   func(eval.Root) (value.Value, bool)
}

// CheckRun is the named one-line check that reported f and the instance it ran on (nil for a
// package check); false when f reports no such run.
type CheckRun func(f diag.Finding) (*syntax.CheckDecl, value.Value, bool)

// Build is the view model of in.Package (VIEWMODEL.md 12), section by section.
func Build(ctx context.Context, in Input) (*vm.ViewModel, error) {
	b, err := newBuilder(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	for _, step := range steps {
		step(b)
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf(fmtWrap, err)
		}
		if b.err != nil {
			return nil, fmt.Errorf(fmtWrap, b.err)
		}
	}
	return b.m, nil
}

// steps fill the sections of the model one after the other; `requires` reads all of them.
var steps = []func(*builder){
	(*builder).types,
	(*builder).views,
	(*builder).values,
	(*builder).usage,
	(*builder).search,
	(*builder).assets,
	(*builder).studioSections,
	(*builder).i18n,
	(*builder).findings,
	(*builder).requires,
}

// builder is one model being built, and what its sections share.
type builder struct {
	ctx     context.Context
	in      Input
	pkg     *check.Package
	m       *vm.ViewModel
	colls   *encode.Colls
	index   *control.Index
	res     *control.Resolver
	texts   *encode.Texts
	defs    *typedef.Types
	render  *render.Renderer
	targets map[string]bool // the lets a ref type of the package targets (S1), once asked
	roots   *encode.Assets  // asset roots by display path (12.3, 12.9)
	err     error           // a section's failure: the drivers' program (12.3)
}

// newBuilder starts the model: every member but `studio` present and empty (J3).
func newBuilder(ctx context.Context, in Input) (*builder, error) {
	pkg := shape.Package(in.Program, in.Package)
	if pkg == nil {
		return nil, fmt.Errorf(fmtPackage, ErrNoPackage, in.Package)
	}
	b := &builder{ctx: ctx, in: in, pkg: pkg, colls: encode.NewColls(force(in.Force)), texts: encode.NewTexts(encode.Catalogues(in.I18N))}
	b.index = control.NewIndex(in.Program, in.Studio)
	b.roots = encode.NewAssets(in.Program, in.Layout)
	tables := table.New(b.index, b.texts)
	b.res = control.NewResolver(b.index, control.Env{
		Counts: b.colls.Counts, Fold: control.FoldWith(ctx, in.Program, in.Fold), Table: tables.Complete,
		Singular: tables.Singular, Assets: b.roots,
	})
	tables.Bind(b.res)
	b.defs = typedef.New(ctx, typedef.Input{
		Program: in.Program, Index: b.index, Colls: b.colls, Texts: b.texts, Assets: b.roots, Fold: in.Fold, Drivers: world(in.Drivers),
	}, in.Package)
	b.render = render.New(ctx, render.Input{Program: in.Program, Index: b.index, Texts: b.texts, Colls: b.colls, Eval: in.Eval})
	b.m = &vm.ViewModel{
		Schema: SchemaVersion, Package: in.Package, Language: in.Language, Requires: []string{},
		Types: map[string]vm.TypeDef{}, Views: map[string]vm.View{}, Values: map[string]vm.Value{},
		Usage: map[string]vm.Usage{}, Search: map[string]vm.SearchIndex{}, Assets: map[string]vm.Asset{},
		Units: map[string]vm.Unit{}, Widgets: map[string]vm.Widget{}, Findings: []vm.Finding{},
	}
	return b, nil
}

// types is the `types` section (12.3).
func (b *builder) types() {
	b.m.Types = b.defs.Section()
	b.err = b.defs.Err()
}

// world is f as typedef resolves it, nil for none.
func world(f func() (*World, error)) typedef.Resolve {
	if f == nil {
		return nil
	}
	return func() (*typedef.World, error) {
		w, err := f()
		if err != nil {
			return nil, err
		}
		return &typedef.World{Program: w.Program, Colls: encode.NewColls(force(w.Force))}, nil
	}
}

// force reads a settled let through f, none when f is nil.
func force(f func(eval.Root) (value.Value, bool)) encode.Force {
	if f == nil {
		return nil
	}
	return func(pkg, name string) (value.Value, bool) { return f(eval.Root{Pkg: pkg, Name: name}) }
}
