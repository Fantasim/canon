package views

import (
	"context"
	"fmt"
	"slices"

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
	Errors    []diag.Finding                      // every package's error findings: views they break render nowhere (J4)
	Eval      render.Evaluator                    // evaluates view expressions; nil: nothing rendered
	CheckRun  CheckRun                            // the check run behind a finding, for its translated messages (J15); nil: none
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
}

// newBuilder starts the model: every member but `studio` present and empty (J3).
func newBuilder(ctx context.Context, in Input) (*builder, error) {
	pkg := shape.Package(in.Program, in.Package)
	if pkg == nil {
		return nil, fmt.Errorf(fmtPackage, ErrNoPackage, in.Package)
	}
	b := &builder{ctx: ctx, in: in, pkg: pkg, colls: encode.NewColls(force(in.Force)), texts: encode.NewTexts(catalogues(in.I18N))}
	b.index = control.NewIndex(in.Program, in.Studio, shape.Errors(slices.Concat(in.Errors, in.Findings)))
	b.roots = encode.NewAssets(in.Program, in.Layout)
	tables := table.New(b.index, b.texts)
	b.res = control.NewResolver(b.index, control.Env{
		Counts: b.colls.Counts, Fold: control.FoldWith(ctx, in.Program, in.Fold), Table: tables.Complete,
		Singular: tables.Singular, Assets: b.roots,
	})
	tables.Bind(b.res)
	b.defs = typedef.New(ctx, typedef.Input{Program: in.Program, Index: b.index, Colls: b.colls, Texts: b.texts, Assets: b.roots, Fold: in.Fold}, in.Package)
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
func (b *builder) types() { b.m.Types = b.defs.Section() }

// catalogues are the key catalogues of results, by package.
func catalogues(results map[string]*i18n.Result) map[string]*i18n.Catalogue {
	out := make(map[string]*i18n.Catalogue, len(results))
	//canon:unordered a map copied into a map
	for pkg, r := range results {
		if r != nil {
			out[pkg] = r.Catalogue
		}
	}
	return out
}

// force reads a settled let through f, none when f is nil.
func force(f func(eval.Root) (value.Value, bool)) encode.Force {
	if f == nil {
		return nil
	}
	return func(pkg, name string) (value.Value, bool) { return f(eval.Root{Pkg: pkg, Name: name}) }
}
