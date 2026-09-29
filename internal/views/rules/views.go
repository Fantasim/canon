package rules

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
)

// Check reports the static view findings of prog into its bags after check (DECISIONS 221):
// studio names project.studio's package ("" for none), emitsView the selected packages that
// have an `emit view`.
func Check(ctx context.Context, prog *check.Program, bags check.Bags, studio string, emitsView map[string]bool) {
	c := newChecker(prog, bags, studio)
	for _, p := range prog.Packages {
		if ctx.Err() != nil {
			return
		}
		bag := bags[p.Path]
		if bag == nil {
			continue
		}
		c.widgetDecls(p, bag)
		c.menuAnnotations(p, bag)
		c.eachView(p, bag, true, (*view).check)
		if emitsView[p.Path] && p.Path != studio {
			c.navigation(p, bag)
		}
	}
}

// checker is one pass over a program's views.
type checker struct {
	info     *check.Info
	studio   *studio // nil when project.studio names no loaded package
	controls *control.Resolver
	first    map[any]source.Span     // each target's first view (E1607)
	labels   map[any]source.Span     // each item's first label (E1614)
	given    map[propKey]source.Span // each item's properties, on whichever line (E1613 twice)
}

func newChecker(prog *check.Program, bags check.Bags, studio string) *checker {
	return &checker{
		info:     prog.Info,
		studio:   studioOf(prog, bags, studio),
		controls: control.NewResolver(control.NewIndex(prog, studio), control.Env{}),
		first:    map[any]source.Span{},
		labels:   map[any]source.Span{},
		given:    map[propKey]source.Span{},
	}
}

// eachView runs fn on each checkable view of p's source files, in path and source order;
// report says whether the target's own findings (E1603, E1607, E1626) are reported.
func (c *checker) eachView(p *check.Package, bag *diag.Bag, report bool, fn func(*view)) {
	for _, f := range p.Files {
		if f.FileKind == syntax.FileSource {
			fileViews(f, func(d *syntax.ViewDecl) {
				v, b := c.viewOf(p.Path, f, d)
				if b != nil && report {
					b.Report(bag)
				}
				if v != nil && !c.broken(v) {
					v.bag = bag
					fn(v)
				}
			})
		}
	}
}

// fileViews calls fn on each view of f, in source order.
func fileViews(f *syntax.File, fn func(*syntax.ViewDecl)) {
	for _, d := range f.Decls {
		if vd, ok := d.(*syntax.ViewDecl); ok {
			fn(vd)
		}
	}
}

// view is one view being checked.
type view struct {
	c       *checker
	pkg     string
	file    *syntax.File
	decl    *syntax.ViewDecl
	bag     *diag.Bag
	target  check.Object
	key     any    // the target: its type, or the define table's let
	owner   string // the package declaring the target
	full    string // the target qualified: b.Item
	kind    targetKind
	name    string // the target as written: Item, Kind.weapon
	fields  []*types.Field
	methods []*types.Method
	variant *types.VariantType
	placed  map[string]source.Span
	items   map[string]source.Span // the view-level items given once (E1605)
	ids     [idKinds]map[string]source.Span
	hides   map[string]bool
	quiet   bool // CheckUnits' pass: the static findings are not reported again
}

func (v *view) span(n syntax.Node) source.Span { return v.file.Span(n) }

// head is the span of n's first token: the word of an item.
func (v *view) head(n syntax.Node) source.Span { return v.tok(n.First()) }

// tok is the span of token t.
func (v *view) tok(t syntax.Tok) source.Span {
	tk := v.file.Tokens[t]
	return source.Span{File: v.file.Src.ID, Start: tk.Start, End: tk.End}
}

func (v *view) report(b *diag.Builder) {
	if !v.quiet {
		b.Report(v.bag)
	}
}

// check checks every item of the view (VIEWMODEL.md §3).
func (v *view) check() {
	for _, it := range v.decl.Items {
		v.item(it)
	}
}
