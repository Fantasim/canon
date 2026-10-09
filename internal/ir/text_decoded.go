package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// TextDecoded are the @text fns of p whose result, a maybe-file's without its `?`, is a map, a list or a keyed list that go types-mode emit e reads, in source order: each has a Decode<Fn>File (DECISIONS 340). Any other result gets none and is never refused, the fn being valid (textDecodable).
func TextDecoded(p *Package, e *Emit) []*ExportFn {
	return slices.DeleteFunc(readableResults(p), func(fn *ExportFn) bool { return !textDecodable(p, e, fn) })
}

// textDecodable reports a readable @text result (readableResults) e's decoders read: every package it reaches, enums and ref targets included, has a go emit e's copy uses (usableGoEmit), and every class of another package it reaches is read by the very checks stage E applies to a types-mode reader (foreignReadable), so that a result they would refuse only goes without a decoder.
func textDecodable(p *Package, e *Emit, fn *ExportFn) bool {
	for pkg := range textReached(p, []*ExportFn{fn}) { //canon:unordered a conjunction: any order gives the same verdict
		if !usableGoEmit(p, pkg) {
			return false
		}
	}
	t := TextResult(fn)
	use := &ForeignUse{variantOf: map[*Case]*Variant{}}
	w := newForeignWalk(p.Name, use)
	w.root(foreignRoot{t: t, read: true})
	return !slices.ContainsFunc(w.out, func(c any) bool { return !foreignReadable(p, e, use, c) })
}

// textReadable reports every part of t, through its elements and keys, a public type a Go reader holds at the top of a file: a scalar, an enum, a ref, a public record or variant, a list, or a map of a readable key; never optional below the top. A local type has no Go type; optional elements or map values, a table, a case, a dependent value, a union not written as a string (a ref union), an unreadable map key or a type of CODEGEN.md §4.4 have no Go reader outside a record.
func textReadable(p *Package, t TypeRef) bool {
	readable := true
	walkTypeRef(t, func(x TypeRef) {
		local := x.Named != nil && pkgOf(x.Named) == p.Name && !slices.Contains(p.Types, x.Named)
		badMap := x.Kind == types.Map && !readableKey(x.Key)
		badUnion := x.Kind == types.LitUnion && x.Elem != nil && x.Elem.Kind != types.String && (x.Elem.Kind != types.Enum || !StringWire(x.Elem))
		if local || badMap || badUnion || !textKinds[x.Kind] {
			readable = false
		}
	})
	return readable
}

// foreignReadable reports class c of another package read by e's readers with no finding: no stored fn or computed default (checkForeignTypesMode's E8014), nothing foreignReadJudges refuse (checkForeignReads), and a make hook its owner writes (checkForeignBuilt's ForeignResolvedRef).
func foreignReadable(p *Package, e *Emit, use *ForeignUse, c any) bool {
	if slices.ContainsFunc(classAndCases(c), typesData) {
		return false
	}
	for _, judge := range foreignReadJudges[e.Target][e.Mode] {
		if _, bad := judge(nil, nil, e, c); bad {
			return false
		}
	}
	return ownerHooks(p, use.classPkg(c), c)
}

// typesData reports a record or case holding what types mode has no data for: a precomputed or lookup fn, a computed default (CODEGEN.md §5.13).
func typesData(class any) bool {
	fields, fns := classBody(class)
	stored := slices.ContainsFunc(fns, func(fn *ExportFn) bool { _, ok := typesFnKinds[fn.Kind]; return ok })
	return stored || slices.ContainsFunc(fields, func(f *Field) bool { return f.Computed })
}

// ownerHooks reports that pkg, the owner of class c, writes c's make hook as p imports its go emit: one exists, and is not a data emit whose loader resolves a ref c holds into a value it selects (GoNamePlan.written, CODEGEN.md §5.14); a types emit selects no value.
func ownerHooks(p *Package, pkg string, c any) bool {
	owner := ownerEmit(p, pkg, TargetGo)
	if owner == nil {
		return false
	}
	if owner.Mode != ModeData {
		return true
	}
	return !slices.ContainsFunc(classAndCases(c), func(class any) bool {
		fields, _ := classBody(class)
		return slices.ContainsFunc(fields, func(f *Field) bool { return f.Input == nil && resolvedBy(owner, pkg, f.Type) })
	})
}

// resolvedBy reports a ref t holds, itself or as list elements, that owner's data loader resolves: into a public let of pkg the emit selects.
func resolvedBy(owner *Emit, pkg string, t TypeRef) bool {
	for t.Kind == types.Optional || t.Kind == types.List {
		if t.Elem == nil {
			return false
		}
		t = *t.Elem
	}
	r := t.Ref
	return t.Kind == types.Ref && r != nil && r.Coll == types.CollLet && !r.Local && r.Pkg == pkg && emitSelects(owner, r.Value)
}

// usableGoEmit reports the go emit of pkg as p imports it, narrowed to the copy the emit uses (CopyOf), one copy under a go_module root: several left means none fits that copy (E8004 noCopy for an ordinary use), an empty import path a root go_module does not map (E8007), none a package without a go emit (E8004).
func usableGoEmit(p *Package, pkg string) bool {
	i := slices.IndexFunc(p.Imports, func(r *PackageRef) bool { return r.Name == pkg })
	if i < 0 || countTarget(p.Imports[i].Emits, TargetGo) != 1 {
		return false
	}
	e := ownerEmit(p, pkg, TargetGo)
	return e != nil && e.GoImport != ""
}
