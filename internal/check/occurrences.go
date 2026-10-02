package check

import (
	"cmp"
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Occurrence is one identifier naming an object (API.md E33, R7); Ambiguous: a field several
// cases of a variant declare, listed under each of them.
type Occurrence struct {
	File      *syntax.File
	Span      source.Span
	Kind      OccKind
	Site      OccSite
	Ambiguous bool
}

// Occurrences lists the occurrences of o Info records, by file path then position (DECISIONS 275).
func (p *Program) Occurrences(o Object) []Occurrence {
	if p == nil || p.Info == nil || o == nil {
		return nil
	}
	p.occOnce.Do(p.indexNames)
	out := slices.Clone(p.occ[o])
	slices.SortFunc(out, func(a, b Occurrence) int {
		return cmp.Or(cmp.Compare(a.File.Src.Path, b.File.Src.Path), cmp.Compare(a.Span.Start, b.Span.Start))
	})
	return out
}

// indexNames walks every file once, listing each identifier Defs, NameUses or Uses records
// under each object it names.
func (p *Program) indexNames() {
	b := &occBuild{info: p.Info, index: map[Object][]Occurrence{}, fields: map[*types.Field]Object{}, handled: map[syntax.Node]bool{}}
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			w := occWalk{occBuild: b, file: f}
			w.walk(f, occCtx{})
		}
	}
	for _, j := range b.paths {
		b.resolvePath(j)
	}
	p.occ = b.index
}

// occBuild is the index being built: the field objects declared, and the amend and template
// paths, listed once every declaration is known, whose names the walk leaves to them.
type occBuild struct {
	info    *Info
	index   map[Object][]Occurrence
	fields  map[*types.Field]Object
	paths   []pathJob
	handled map[syntax.Node]bool
}

// occWalk lists the occurrences of one file.
type occWalk struct {
	*occBuild
	file *syntax.File
}

// occCtx is what an identifier's place says of it: its parent and grandparent nodes, its site,
// and the kind every name under an emit takes.
type occCtx struct {
	parent, grand syntax.Node
	site          OccSite
	forced        OccKind
	isForced      bool
}

func (w *occWalk) walk(n syntax.Node, ctx occCtx) {
	switch x := n.(type) {
	case *syntax.Amendment:
		w.amendPath(x, ctx)
	case *syntax.Annotation:
		w.templatePaths(x, ctx)
	case *syntax.Ident:
		w.ident(x, ctx)
	case *syntax.IdentExpr:
		if o := w.info.Uses[x]; o != nil && !w.handled[x] {
			w.add(o, Occurrence{Span: w.file.Span(x), Kind: ctx.kindOr(OccUse), Site: ctx.site})
		}
	}
	inner := ctx.under(n)
	for c := range syntax.Children(n) {
		w.walk(c, inner)
	}
}

// ident lists a declaring identifier, and the object it names when that is another one.
func (w *occWalk) ident(id *syntax.Ident, ctx occCtx) {
	def := w.info.Defs[id]
	if def != nil {
		w.add(def, Occurrence{Span: w.file.Span(id), Kind: OccDecl, Site: ctx.site})
		w.noteField(def)
	}
	if o := w.info.NameUses[id]; o != nil && o != def && !w.handled[id] {
		w.add(o, Occurrence{Span: w.file.Span(id), Kind: ctx.kindOr(ctx.nameKind(w.info, id)), Site: ctx.site})
	}
}

func (w *occWalk) add(o Object, occ Occurrence) {
	occ.File = w.file
	w.index[o] = append(w.index[o], occ)
}

// under is the context of n's children: a view or a translation, a check and an amend block set
// the site as API.md R7 reads it; an emit's names are its values.
func (ctx occCtx) under(n syntax.Node) occCtx {
	next := ctx
	next.parent, next.grand = n, ctx.parent
	switch n.(type) {
	case *syntax.ViewDecl, *syntax.TranslationEntry:
		next.site = SiteView
	case *syntax.CheckDecl:
		next.site = SiteCheck
	case *syntax.AmendBlock:
		next.site = SiteLayer
	case *syntax.EmitDecl:
		next.forced, next.isForced = OccEmitValues, true
	}
	return next
}

// kindOr is the kind forced on the names under an emit, else k.
func (ctx occCtx) kindOr(k OccKind) OccKind {
	if ctx.isForced {
		return ctx.forced
	}
	return k
}

// nameKind is how an identifier that NameUses records names its object, by its parent node.
func (ctx occCtx) nameKind(info *Info, id *syntax.Ident) OccKind {
	switch p := ctx.parent.(type) {
	case *syntax.QualifiedName:
		return qualifiedKind(ctx.grand, p, id)
	case *syntax.SelectorExpr:
		if p.X == nil || info.Selections[p] != nil {
			return OccSelector
		}
		return OccQualified
	case *syntax.Import:
		return OccImport
	case *syntax.FieldItem:
		return OccLiteralField
	case *syntax.Arg:
		return OccNamedArg
	case *syntax.KeyedType:
		return OccKeyedBy
	case *syntax.EntryDecl:
		return OccEntry
	case *syntax.AmendBlock, *syntax.AmendSegment:
		return OccAmend
	case *syntax.ViewDecl, *syntax.ViewField, *syntax.ViewColumn, *syntax.ViewFilter, *syntax.ViewMenu,
		*syntax.ViewShow, *syntax.ViewGroup:
		return OccViewItem
	}
	return OccUse
}

// qualifiedKind is the kind of a part of a dotted name, by the node holding the name: a name
// qualified by another part is a qualified use.
func qualifiedKind(holder syntax.Node, q *syntax.QualifiedName, id *syntax.Ident) OccKind {
	switch holder.(type) {
	case *syntax.Import:
		return OccImport
	case *syntax.TranslationEntry:
		return OccTranslationKey
	case *syntax.Pattern, *syntax.IsExpr:
		return OccPattern
	case *syntax.RefType:
		return OccRef
	}
	if q.Parts[0] != id {
		return OccQualified
	}
	return OccUse
}
