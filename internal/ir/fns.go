package ir

import (
	"path"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/wire"
)

// fnSite is an export fn with what stage E needs to compute and judge it.
type fnSite struct {
	fn    *ExportFn
	obj   check.Object
	decl  *syntax.FnDecl
	sig   *types.FuncType
	label string     // the fn in messages: `canTransition`, `Potion.healFor`
	pkg   string     // the declaring package
	recv  types.Type // the record or case of a method; nil for a package fn
	text  bool       // a `@text` fn: a file, not API (CODEGEN.md §2.9)
}

// methods are the export methods of a record or case body but the broken ones (decisions 209, 213), in declaration order; owner is the record or case as messages name it (`Potion`, `Reward.item`).
func (s *stage) methods(ms []*types.Method, items []syntax.RecordItem, owner string, recv types.Type) []*ExportFn {
	var out []*ExportFn
	for _, m := range ms {
		if !m.Export || m.Type == nil {
			continue
		}
		d := methodDecl(items, m.Name)
		if d == nil {
			continue
		}
		obj := s.info.Defs[d.Name]
		if obj == nil || s.info.Broken[obj] {
			continue
		}
		site := s.exportFn(obj, d, m.Type)
		site.label, site.recv = owner+qnameSep+m.Name, recv
		out = append(out, site.fn)
	}
	return out
}

func methodDecl(items []syntax.RecordItem, name string) *syntax.FnDecl {
	for _, it := range items {
		if d, ok := it.(*syntax.FnDecl); ok && d.Name != nil && d.Name.Name == name {
			return d
		}
	}
	return nil
}

// exportFn is the static IR of an export fn: signature, kind and name overrides (SPEC §9.4).
func (s *stage) exportFn(obj check.Object, d *syntax.FnDecl, sig *types.FuncType) *fnSite {
	fn := &ExportFn{Name: obj.Name(), Result: s.ref(sig.Result), Kind: kindOf(sig)}
	if d.Doc != nil {
		fn.Doc = d.Doc.Text
	}
	if f := obj.File(); f != nil && f.Src != nil {
		fn.File = path.Base(f.Src.Path)
	}
	fn.ResultRange, _ = ownRefinements(sig.Result)
	for i, p := range sig.Params {
		param := &Param{Type: s.ref(p)}
		param.Range, _ = ownRefinements(p)
		if i < len(d.Params) && d.Params[i].Name != nil {
			param.Name = d.Params[i].Name.Name
			s.nodeSites[param] = declSite{file: obj.File(), node: d.Params[i].Name}
		}
		fn.Params = append(fn.Params, param)
	}
	n := nameOverrides(d.Annotations)
	fn.Go, fn.Cpp, fn.TS = n.goName, n.cpp, n.ts
	site := &fnSite{fn: fn, obj: obj, decl: d, sig: sig, label: obj.Name(), pkg: obj.Pkg()}
	s.fnObjs[fn] = site
	s.fnByObj[obj] = site
	return site
}

// orderFns numbers u's export fns in declaration order, methods and package fns together, so a conformance file lists them as declared (CONFORMANCE.md §7.2).
func (s *stage) orderFns(u *unit) {
	n := 0
	for _, obj := range u.cp.Decls {
		for _, d := range fnDecls(obj.Decl()) {
			if site := s.fnByObj[s.info.Defs[d.Name]]; site != nil {
				site.fn.Order = n
				n++
			}
		}
	}
}

// fnDecls are the named fn declarations of a top-level declaration: itself, or those of its record or case bodies, in order.
func fnDecls(d syntax.Node) []*syntax.FnDecl {
	switch x := d.(type) {
	case *syntax.FnDecl:
		return bodyFns([]syntax.RecordItem{x})
	case *syntax.RecordDecl:
		if x.Body != nil {
			return bodyFns(x.Body.Items)
		}
	case *syntax.VariantDecl:
		var out []*syntax.FnDecl
		for _, it := range x.Items {
			if c, ok := it.(*syntax.VariantCase); ok && c.Body != nil {
				out = append(out, bodyFns(c.Body.Items)...)
			}
		}
		return out
	}
	return nil
}

// bodyFns are the named fn declarations among items, in order.
func bodyFns(items []syntax.RecordItem) []*syntax.FnDecl {
	var out []*syntax.FnDecl
	for _, it := range items {
		if fd, ok := it.(*syntax.FnDecl); ok && fd.Name != nil {
			out = append(out, fd)
		}
	}
	return out
}

// kindOf is how an export fn is emitted (SPEC §9.4): no parameter, only finite ones (wire.Stored, WIRE.md §5.11), else translated.
func kindOf(sig *types.FuncType) FnKind {
	switch {
	case len(sig.Params) == 0:
		return FnPrecomputed
	case wire.Stored(sig):
		return FnLookup
	}
	return FnTranslated
}
