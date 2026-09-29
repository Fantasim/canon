package verify

import (
	"path"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// written is where a type is written: the declaration holding it, the type itself (a dependent
// type's `dep`, ERRORS.md E3801, E3802), and a refinement's bound or predicate.
type written struct {
	decl, typ, arg source.Span
}

// sources locates what a finding relates: the written types Info.TypeExprs resolves, the
// declarations of enums and variants, and the directory each asset root is written in.
type sources struct {
	types     map[types.Type]written
	decls     map[syntax.Node]*syntax.File
	assetDirs map[*types.AssetSpec]string // the declaring file's directory, project-relative (WIRE.md §2.2 rule 1)
}

// indexSources inverts Info.TypeExprs for the type pointers written at one place, each file's
// syntax scanned once per cache.
func indexSources(prog *check.Program, c *IndexCache) *sources {
	s := &sources{types: map[types.Type]written{}, decls: map[syntax.Node]*syntax.File{}, assetDirs: map[*types.AssetSpec]string{}}
	if prog == nil || prog.Info == nil {
		return s
	}
	for _, pkg := range prog.Packages {
		for _, f := range pkg.Files {
			s.add(f, c.scanned(f), prog.Info)
		}
	}
	return s
}

// add indexes one file from its scan, each type at its first written place.
func (s *sources) add(f *syntax.File, fs *fileSyntax, info *check.Info) {
	for _, d := range fs.decls {
		s.decls[d] = f
	}
	for _, a := range fs.assets {
		s.addAsset(f, info.TypeExprs[a])
	}
	for _, at := range fs.types {
		typ := info.TypeExprs[at.t]
		if _, seen := s.types[typ]; writtenOnce(typ) && !seen {
			s.types[typ] = at.w
		}
	}
}

// fileSyntax is what indexing reads of one file's tree, in source order: the enum and variant
// declarations, the asset types, and each written type with where it is written.
type fileSyntax struct {
	decls  []syntax.Node
	assets []*syntax.AssetType
	types  []typeAt
}

// typeAt is a written type and its places.
type typeAt struct {
	t syntax.Type
	w written
}

// scanFile walks f once for its fileSyntax.
func scanFile(f *syntax.File) *fileSyntax {
	out := &fileSyntax{}
	named := map[syntax.Type]*syntax.Ident{}
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch d := n.(type) {
		case *syntax.EnumDecl, *syntax.VariantDecl:
			out.decls = append(out.decls, d)
		case *syntax.AssetType:
			out.assets = append(out.assets, d)
		}
		if name, t := declared(n); t != nil {
			named[t] = name
		}
		if t, isType := n.(syntax.Type); isType {
			out.types = append(out.types, typeAt{t: t, w: written{decl: declSpan(f, named[t], t), typ: f.Span(t), arg: f.Span(argOf(t))}})
		}
		return true
	})
	return out
}

// addAsset records the directory of the file an asset root is written in; a broken asset type
// is String, with no root.
func (s *sources) addAsset(f *syntax.File, t types.Type) {
	if r, ok := t.(*types.Refined); ok && r.Asset != nil {
		s.assetDirs[r.Asset] = path.Dir(f.Src.Path)
	}
}

// writtenOnce: a type the checker builds where it is written, not a declaration it interns.
func writtenOnce(t types.Type) bool {
	switch t.(type) {
	case *types.Refined, *types.RefType, *types.ListType, *types.MapType, *types.DepMapType,
		*types.TableType, *types.OptionalType, *types.LitUnionType, *types.TypeAppType, *types.AppliedRecord:
		return true
	}
	return false
}

// related relates the written type t a value was checked against, when it is known.
func (s *sources) related(b *diag.Builder, t types.Type) *diag.Builder {
	if w, ok := s.types[t]; ok {
		b.Related(w.decl, diag.NoteSource(w.decl))
	}
	return b
}

// relatedNode relates a declaration node of an enum or a variant.
func (s *sources) relatedNode(b *diag.Builder, decl syntax.Node, n syntax.Node) *diag.Builder {
	if f := s.decls[decl]; f != nil && n != nil {
		b.Related(f.Span(n), diag.NoteSource(f.Span(n)))
	}
	return b
}

// declared is the name and written type of a field or an annotated let.
func declared(n syntax.Node) (*syntax.Ident, syntax.Type) {
	switch d := n.(type) {
	case *syntax.FieldDecl:
		return d.Name, d.Type
	case *syntax.LetDecl:
		return d.Name, d.Type
	}
	return nil, nil
}

// declSpan covers a declaration's name and type, `label: String(1..)` (ERRORS.md §1.5 source).
func declSpan(f *syntax.File, name *syntax.Ident, t syntax.Type) source.Span {
	if name == nil {
		return f.Span(t)
	}
	return f.Span(name).Cover(f.Span(t))
}

// argOf is the part of a written refinement E3204 and E3206 quote: the range or the predicate.
func argOf(t syntax.Type) syntax.Node {
	var args *syntax.TypeArgs
	switch t := t.(type) {
	case *syntax.WhereType:
		return t.Pred
	case *syntax.NamedType:
		args = t.Args
	case *syntax.ListType:
		args = t.Args
	case *syntax.MapType:
		args = t.Args
	case *syntax.DepMapType:
		args = t.Args
	}
	if args != nil {
		for _, a := range args.Args {
			if _, isRegex := a.(*syntax.RegexLit); !isRegex {
				return a
			}
		}
	}
	return t
}

// memberNode is the declaration of an enum member, by name.
func memberNode(e *types.EnumType, name string) syntax.Node {
	if e.Decl == nil {
		return nil
	}
	for _, m := range e.Decl.Members {
		if m != nil && m.Name != nil && m.Name.Name == name {
			return m
		}
	}
	return nil
}

// caseNode is the declaration of a variant case, by name.
func caseNode(v *types.VariantType, name string) syntax.Node {
	if v.Decl == nil {
		return nil
	}
	for _, item := range v.Decl.Items {
		if c, ok := item.(*syntax.VariantCase); ok && c.Name != nil && c.Name.Name == name {
			return c
		}
	}
	return nil
}

// DeclaredTypes is the declared type of every top-level value (EVALUATION.md §2.1).
func DeclaredTypes(prog *check.Program) map[eval.Root]types.Type {
	out := map[eval.Root]types.Type{}
	if prog == nil {
		return out
	}
	for _, pkg := range prog.Packages {
		for _, obj := range pkg.Decls {
			if k := obj.Kind(); k == check.ObjLet || k == check.ObjConst {
				out[eval.Root{Pkg: pkg.Path, Name: obj.Name()}] = obj.Type()
			}
		}
	}
	return out
}
