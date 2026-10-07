package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// ReportSilenced reports W4001: per broken record or variant, the selected values whose type reaches it (EVALUATION.md §1).
func ReportSilenced(prog *check.Program, selected []*check.Package, bags check.Bags) {
	if prog == nil || prog.Info == nil {
		return
	}
	s := &silencer{info: prog.Info, types: brokenTypes(prog), counts: map[check.Object]int64{}, bagOf: map[check.Object]string{}}
	if len(s.types) == 0 {
		return
	}
	chosen := map[string]bool{}
	for _, pkg := range selected {
		chosen[pkg.Path] = true
		for _, obj := range pkg.Decls {
			s.count(pkg.Path, obj)
		}
	}
	for _, obj := range s.order {
		pkg := obj.Pkg()
		if !chosen[pkg] { // a broken type of an import: the first selected package it silences shows it
			pkg = s.bagOf[obj]
		}
		if bag := bags[pkg]; bag != nil {
			silencedBy(obj, s.counts[obj]).Report(bag)
		}
	}
}

// silencer counts, per broken record or variant declaration, the selected values reaching it.
type silencer struct {
	info   *check.Info
	types  map[types.Type]check.Object // each broken record or variant, by its type
	counts map[check.Object]int64
	bagOf  map[check.Object]string // the first selected package holding a value it silences
	order  []check.Object          // the broken declarations, in the order a first value counted them
}

// brokenTypes maps the type of every broken record or variant declaration of prog to it (DECISIONS 209).
func brokenTypes(prog *check.Program) map[types.Type]check.Object {
	out := map[types.Type]check.Object{}
	for _, pkg := range prog.Packages {
		for _, obj := range pkg.Decls {
			if obj.Kind() == check.ObjTypeName && prog.Info.Broken[obj] && declName(obj) != nil && obj.Type() != nil {
				out[obj.Type().Base()] = obj
			}
		}
	}
	return out
}

// declName is the name of a record or variant declaration, nil for any other declaration.
func declName(obj check.Object) *syntax.Ident {
	switch d := obj.Decl().(type) {
	case *syntax.RecordDecl:
		return d.Name
	case *syntax.VariantDecl:
		return d.Name
	}
	return nil
}

// count adds obj, a top-level value of pkg left unevaluated, to every broken type its type reaches.
func (s *silencer) count(pkg string, obj check.Object) {
	if obj.Kind() != check.ObjConst && obj.Kind() != check.ObjLet || !s.info.Broken[obj] || obj.Type() == nil {
		return
	}
	for _, d := range s.reached(obj.Type()) {
		if s.counts[d] == 0 {
			s.order = append(s.order, d)
			s.bagOf[d] = pkg
		}
		s.counts[d]++
	}
}

// reached is the broken record and variant declarations t reaches through elements, entries,
// cases, fields and dependent branches, never through a ref, each once.
func (s *silencer) reached(t types.Type) []check.Object {
	var out []check.Object
	seen := map[types.Type]bool{}
	todo := []types.Type{t}
	for len(todo) > 0 {
		next := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if next == nil || seen[next.Base()] {
			continue
		}
		next = next.Base()
		seen[next] = true
		if d, ok := s.types[typedDecl(next)]; ok && !slices.Contains(out, d) {
			out = append(out, d)
		}
		todo = append(todo, partsOf(next)...)
	}
	return out
}

// typedDecl is the record or variant whose declaration types the values of t: an applied
// record's record, a case's variant, else t.
func typedDecl(t types.Type) types.Type {
	if c, ok := t.(*types.CaseType); ok {
		return c.Variant
	}
	return declOf(t)
}

// partsOf is what a value of t holds: fields, cases, elements, entries, keys, values and branches.
func partsOf(t types.Type) []types.Type {
	switch d := t.(type) {
	case *types.VariantType:
		out := make([]types.Type, len(d.Cases))
		for i, c := range d.Cases {
			out[i] = c
		}
		return out
	case *types.TypeAppType:
		return types.Branches(d.Fn)
	case *types.DepUnionType:
		return types.Branches(d.Fn)
	case *types.OptionalType:
		return []types.Type{d.Elem}
	case *types.ListType:
		return []types.Type{d.Elem}
	case *types.TableType:
		return []types.Type{d.Elem}
	case *types.MapType:
		return []types.Type{d.Key, d.Value}
	case *types.DepMapType:
		return []types.Type{d.Value}
	case *types.PairType:
		return []types.Type{d.A, d.B}
	}
	fs := fieldsOf(t)
	out := make([]types.Type, len(fs))
	for i, f := range fs {
		out[i] = f.Type
	}
	return out
}

// silencedBy is W4001 at obj's name for n values (ERRORS.md: one, many).
func silencedBy(obj check.Object, n int64) *diag.Builder {
	span, typ := obj.File().Span(declName(obj)), obj.Type().Base().String()
	if n == 1 {
		return diag.W4001.AtOne(span, typ)
	}
	return diag.W4001.AtMany(span, n, typ)
}
