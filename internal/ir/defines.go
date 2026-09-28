package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// DefineTarget is the load.defines table a field of type t refs, itself or as its list's elements; nil for any other type, a ref inside a map or a nested list included (CODEGEN.md §5.8).
func DefineTarget(t TypeRef) *RefTarget {
	if t.Kind == types.List && t.KeyedBy == nil && t.Elem != nil {
		t = *t.Elem
	}
	if DefinesRef(t) {
		return t.Ref
	}
	return nil
}

// DefinesOf is the table of p.Defines that r targets, nil when p holds none.
func DefinesOf(p *Package, r *RefTarget) *DefineTable {
	if r == nil {
		return nil
	}
	i := slices.IndexFunc(p.Defines, func(d *DefineTable) bool { return d.Pkg == r.Pkg && d.Value == r.Value })
	if i < 0 {
		return nil
	}
	return p.Defines[i]
}

// OwnDefines are the define tables the fields of p's own classes ref, in p.Defines' order: one table per emit each (CODEGEN.md §5.8).
func OwnDefines(p *Package) []*DefineTable {
	refs := ownDefineRefs(p)
	var out []*DefineTable
	for _, d := range p.Defines {
		if slices.ContainsFunc(refs, func(r *DefineTable) bool { return r.Pkg == d.Pkg && r.Value == d.Value }) {
			out = append(out, d)
		}
	}
	return out
}

// ownDefineRefs are the define tables the fields of p's own classes ref, by package and let, their values aside: what the name plans declare, whether or not stage E could read the tables.
func ownDefineRefs(p *Package) []*DefineTable {
	var out []*DefineTable
	for _, class := range packageClasses(p) {
		fields, _ := classBody(class)
		for _, f := range fields {
			r := DefineTarget(f.Type)
			if r != nil && f.Input == nil && !slices.ContainsFunc(out, func(d *DefineTable) bool { return d.Pkg == r.Pkg && d.Value == r.Value }) {
				out = append(out, &DefineTable{Pkg: r.Pkg, Value: r.Value})
			}
		}
	}
	return out
}

// DefineTableName is <Table> of a define table, UpperCamel of its let (CODEGEN.md §3.2, §5.8), as both loaders' error names it.
func DefineTableName(d *DefineTable) string { return cppUpperCamel(d.Value) }

// defineSuffix is a define value getter's suffix after the key's getter: Value, Values for a list (CODEGEN.md §3.3, §5.8).
func defineSuffix(list bool) string {
	if list {
		return defineValueSuffix + goPluralSuffix
	}
	return defineValueSuffix
}
