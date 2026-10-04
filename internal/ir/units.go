package ir

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// packageDir is the display path of a package's directory, `/`-separated (CODEGEN.md §2.4).
func packageDir(cp *check.Package) string {
	for _, f := range sourceFiles(cp) {
		if f.Src != nil {
			return path.Dir(f.Src.Path)
		}
	}
	return ""
}

// packageDoc is the package doc of every source file in path order, joined by a blank line (GRAMMAR.md §9.1).
func packageDoc(cp *check.Package) string {
	var docs []string
	for _, f := range sourceFiles(cp) {
		if f.Doc != nil && f.Doc.Text != "" {
			docs = append(docs, f.Doc.Text)
		}
	}
	return strings.Join(docs, docSep)
}

// local reports a `local` declaration; everything else is public.
func local(m *syntax.Modifiers) bool {
	return m != nil && m.Local.Valid()
}

// exported reports an `export fn` (SPEC §9.4).
func exported(d *syntax.FnDecl) bool { return d.Mods != nil && d.Mods.Export.Valid() }

// assemble builds u's types, constants, values and package fns, each list in declaration order (CODEGEN.md §2.7): every public type, constant and export fn, and every public value.
func (s *stage) assemble(u *unit) {
	for _, obj := range u.cp.Decls {
		s.assembleDecl(u, obj)
	}
	for _, v := range u.values {
		u.p.Values = append(u.p.Values, v.v)
	}
}

// assembleDecl adds one top-level declaration of u, if it is public (CODEGEN.md §2.7) and not broken: a broken declaration never enters the emit IR (decision 213), and everything naming it is broken too (TYPES.md §1, decision 209).
func (s *stage) assembleDecl(u *unit, obj check.Object) {
	if s.info.Broken[obj] {
		return
	}
	switch d := obj.Decl().(type) {
	case *syntax.RecordDecl, *syntax.EnumDecl, *syntax.VariantDecl, *syntax.TypeDecl:
		if t := s.publicType(obj); t != nil {
			u.p.Types = append(u.p.Types, t)
		}
	case *syntax.ConstDecl:
		s.assembleConst(u, obj, d)
	case *syntax.LetDecl:
		s.assembleLet(u, obj, d)
	case *syntax.FnDecl:
		s.assembleFn(u, obj, d)
	}
}

func (s *stage) assembleConst(u *unit, obj check.Object, d *syntax.ConstDecl) {
	if local(d.Mods) {
		return
	}
	c := &constSite{c: s.constant(obj, d), obj: obj, decl: d}
	s.nodeSites[c.c] = declSite{file: obj.File(), node: d.Name}
	u.consts = append(u.consts, c)
	u.p.Consts = append(u.p.Consts, c.c)
}

func (s *stage) assembleLet(u *unit, obj check.Object, d *syntax.LetDecl) {
	if local(d.Mods) {
		return
	}
	v := s.letValue(obj, d)
	s.nodeSites[v.v] = v.span()
	u.values = append(u.values, v)
}

func (s *stage) assembleFn(u *unit, obj check.Object, d *syntax.FnDecl) {
	sig, ok := obj.Type().(*types.FuncType)
	if !ok || !exported(d) {
		return
	}
	site := s.exportFn(obj, d, sig)
	u.fns = append(u.fns, site)
	if annotation(d.Annotations, syntax.AnnText) != nil {
		u.p.TextFns = append(u.p.TextFns, site.fn)
		return
	}
	u.p.Fns = append(u.p.Fns, site.fn)
}

// publicType is the IR of a public record, enum, variant or dependent type; an alias and a type function without a match have no name in generated code (CODEGEN.md §4.4).
func (s *stage) publicType(obj check.Object) Type {
	if isLocalDecl(obj.Decl()) {
		return nil
	}
	switch t := obj.Type().(type) {
	case *types.RecordType:
		return s.record(t)
	case *types.EnumType:
		return s.enum(t)
	case *types.VariantType:
		return s.variant(t)
	case *types.DepUnionType:
		if t.Fn != nil && t.Fn.Scrutinee != nil {
			return s.dependent(t.Fn)
		}
	}
	return nil
}

func isLocalDecl(n syntax.Node) bool {
	switch d := n.(type) {
	case *syntax.RecordDecl:
		return local(d.Mods)
	case *syntax.EnumDecl:
		return local(d.Mods)
	case *syntax.VariantDecl:
		return local(d.Mods)
	case *syntax.TypeDecl:
		return local(d.Mods)
	}
	return false
}

// constant is a public const and its value.
func (s *stage) constant(obj check.Object, d *syntax.ConstDecl) *Const {
	c := s.constDecl(obj, d)
	if v, ok := s.in.Host.Value(s.ctx, obj.Pkg(), obj.Name()); ok {
		c.V = v
	}
	return c
}

// letValue is a public let: its declared type, value, @reload and table ids (EMT-04).
func (s *stage) letValue(obj check.Object, d *syntax.LetDecl) *valueSite {
	v := s.letDecl(obj, d)
	if val, ok := s.in.Host.Value(s.ctx, obj.Pkg(), obj.Name()); ok {
		v.V = val
		v.IDs = tableIDs(val)
	}
	return &valueSite{v: v, obj: obj, decl: d, t: obj.Type()}
}

// constDecl is a public const as declared, without its value.
func (s *stage) constDecl(obj check.Object, d *syntax.ConstDecl) *Const {
	c := &Const{Name: obj.Name(), Doc: docOf(d.Doc), Type: s.ref(obj.Type())}
	n := nameOverrides(d.Annotations)
	c.Go, c.Cpp, c.TS = n.goName, n.cpp, n.ts
	return c
}

// letDecl is a public let as declared: its type, @reload and name overrides, without its value.
func (s *stage) letDecl(obj check.Object, d *syntax.LetDecl) *Value {
	v := &Value{Name: obj.Name(), Doc: docOf(d.Doc), Type: s.ref(obj.Type()), Reload: annotation(d.Annotations, syntax.AnnReload) != nil}
	n := nameOverrides(d.Annotations)
	v.Go, v.Cpp, v.TS = n.goName, n.cpp, n.ts
	return v
}

func docOf(d *syntax.DocComment) string {
	if d == nil {
		return ""
	}
	return d.Text
}
