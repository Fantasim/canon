package ir

import "github.com/fantasim/canonlang/internal/types"

// GoDependent are a dependent type's Go names (CODEGEN.md §3.3, §5.6): its struct, its branch enum TBranch, the struct's Branch method and storage, and each branch's names in arm order.
type GoDependent struct {
	Type, Branch, Method, BranchStore, ValueStore string
	Branches                                      []GoBranch
}

// GoBranch is one branch's TBranch member, its As<Branch> and, for a ref into a load.defines table, its As<Branch>Value ("" else).
type GoBranch struct {
	Member, As, AsValue string
}

// Dependent is d's Go names: a @go(name:) override replaces T in every name built on it (CODEGEN.md §3.5).
func (pl *GoNamePlan) Dependent(d *Dependent) GoDependent {
	name := pl.TypeName(d)
	out := GoDependent{Type: name, Branch: name + branchWord, Method: branchWord, BranchStore: goBranchStore, ValueStore: GoCaseStore}
	for _, b := range d.Branches {
		br := GoBranch{Member: out.Branch + goUpperCamel(b.Name), As: goAsPrefix + goUpperCamel(b.Name)}
		if DefinesRef(b.Type) {
			br.AsValue = br.As + asValueSuffix
		}
		out.Branches = append(out.Branches, br)
	}
	return out
}

// DefinesRef reports a ref into a load.defines table, whose accessor also exposes the define's value (CODEGEN.md §5.6, §5.8).
func DefinesRef(t TypeRef) bool {
	return t.Kind == types.Ref && t.Ref != nil && t.Ref.Coll == types.CollDefines
}

// declareDependent declares a dependent type's struct, its branch enum and members in the package, then the struct's methods before its storage (decision 203).
func (pl *GoNamePlan) declareDependent(top *nameScope, d *Dependent) {
	n, origin := pl.Dependent(d), d.QName()
	pl.declare(top, n.Type, origin, d)
	pl.declareFrom(top, origin, d, derivation{d, func() string { return pl.Dependent(d).Branch }})
	for i := range n.Branches {
		pl.declareFrom(top, origin+qnameSep+d.Branches[i].Name, d, derivation{d, func() string { return pl.Dependent(d).Branches[i].Member }})
	}
	sc := pl.scope(n.Type)
	pl.declare(sc, n.Method, origin, d)
	for i, b := range n.Branches {
		pl.declare(sc, b.As, origin+qnameSep+d.Branches[i].Name, d)
		if b.AsValue != "" {
			pl.declare(sc, b.AsValue, origin+qnameSep+d.Branches[i].Name, d)
		}
	}
	pl.declare(sc, n.BranchStore, origin, d)
	pl.declare(sc, n.ValueStore, origin, d)
}

// heldDependents are the dependent types a decoded class's fields hold, through lists and optionals: a data loader decodes them with decode<T> (CODEGEN.md §5.6, §6.1).
func heldDependents(class any) []*Dependent {
	fields, _ := classBody(class)
	var out []*Dependent
	for _, f := range fields {
		t := &f.Type
		for t.Elem != nil && (t.Kind == types.List || t.Kind == types.Optional) {
			t = t.Elem
		}
		if d, ok := t.Named.(*Dependent); ok && f.Pairs == nil && f.Input == nil && t.Kind == types.TypeApp {
			out = append(out, d)
		}
	}
	return out
}

// declareDependentDecoders declares decode<T> of each own dependent type a decoded class holds, in declaration order.
func (pl *GoNamePlan) declareDependentDecoders(top *nameScope) {
	for _, t := range pl.p.Types {
		if d, ok := t.(*Dependent); ok && pl.Decoded(d) {
			pl.declare(top, pl.DecodeFunc(d), pl.goNameOf[d], d)
		}
	}
}
