package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// TextDecoded are the @text fns of p whose result, a maybe-file's without its `?`, is a map, a list or a keyed list that the types-mode code emit e reads, in source order: each has a Decode<Fn>File, in TypeScript decode<Fn>File and parse<Fn>File (DECISIONS 340, 363). Any other result gets none and is never refused, the fn being valid (textDecodable); none in another mode.
func TextDecoded(p *Package, e *Emit) []*ExportFn {
	if !isCode(e.Target) || e.Mode != ModeTypes {
		return nil
	}
	return slices.DeleteFunc(readableResults(p), func(fn *ExportFn) bool { return !textDecodable(p, e, fn) })
}

// textDecodable reports a readable @text result (readableResults) e's decoders read: every package it reaches, enums and ref targets included, has an emit of e's target e's copy uses (usableEmit), and every class of another package it reaches is read by the very checks stage E applies to a types-mode reader of that target (foreignReadable), so that a result they would refuse only goes without a decoder.
func textDecodable(p *Package, e *Emit, fn *ExportFn) bool {
	for pkg := range textReached(p, []*ExportFn{fn}) { //canon:unordered a conjunction: any order gives the same verdict
		if !usableEmit(p, pkg, e.Target) {
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

// foreignReadable reports class c of another package read by e's readers with no finding: no stored fn or computed default (checkForeignTypesMode's E8014), nothing foreignReadJudges refuse (checkForeignReads, a map in C++ types mode included), and a make hook its owner writes (checkForeignBuilt's ForeignResolvedRef).
func foreignReadable(p *Package, e *Emit, use *ForeignUse, c any) bool {
	if slices.ContainsFunc(classAndCases(c), typesData) {
		return false
	}
	for _, judge := range foreignReadJudges[e.Target][e.Mode] {
		if _, bad := judge(nil, nil, e, c); bad {
			return false
		}
	}
	return ownerHooks(p, e.Target, use, c)
}

// typesData reports a record or case holding what types mode has no data for: a precomputed or lookup fn, a computed default (CODEGEN.md §5.13).
func typesData(class any) bool {
	fields, fns := classBody(class)
	stored := slices.ContainsFunc(fns, func(fn *ExportFn) bool { _, ok := typesFnKinds[fn.Kind]; return ok })
	return stored || slices.ContainsFunc(fields, func(f *Field) bool { return f.Computed })
}

// ownerHooks reports that the owner of class c writes c's make hook, each case's for a variant, as p imports its emit of target t: one exists and its NoHook, filled from the owner's own plan, names none of them (CODEGEN.md §5.14, DECISIONS 363); TypeScript has no make hooks, its readers write object literals.
func ownerHooks(p *Package, t Target, use *ForeignUse, c any) bool {
	owner := ownerEmit(p, use.classPkg(c), t)
	if owner == nil || t == TargetTS {
		return owner != nil
	}
	return !slices.ContainsFunc(classAndCases(c), func(class any) bool { return slices.Contains(owner.NoHook, hookName(use, class)) })
}

// hookName is how NoHook names a record, Name, or a case, Variant.Case (DECISIONS 363).
func hookName(use *ForeignUse, class any) string {
	switch x := class.(type) {
	case *Record:
		return x.Name
	case *Case:
		if v := use.VariantOf(x); v != nil {
			return v.Name + qnameSep + x.Name
		}
	}
	return ""
}

// usableEmit reports the emit of target t of pkg as p imports it, narrowed to the copy the emit uses (CopyOf): one copy, and for go one under a go_module root. Several left means none fits that copy (E8004 noCopy for an ordinary use), an empty import path a root go_module does not map (E8007), none a package without an emit of t (E8004) or a copy under another consumer root (E8004 consumerCopy).
func usableEmit(p *Package, pkg string, t Target) bool {
	i := slices.IndexFunc(p.Imports, func(r *PackageRef) bool { return r.Name == pkg })
	if i < 0 || countTarget(p.Imports[i].Emits, t) != 1 {
		return false
	}
	e := ownerEmit(p, pkg, t)
	return e != nil && (t != TargetGo || e.GoImport != "")
}
