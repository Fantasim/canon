package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// CppHook is a make hook, a static member of detail::<P>Make (CODEGEN.md §5.14): Name builds the value (T, V_<Case>), CaseType a case with fields' class alone (Case_V_<Case>, "" else); Members are the class's storage in member order, never id_ and retired_, which a table's rows fill. Written is false for a class of this package whose data or types-mode loader resolves one of its refs: its names stay reserved, no hook is written (log-2026-10-06 "U1 review" 3); another package's class reads true, its owner's refusal being stage E's.
type CppHook struct {
	Name, CaseType string
	Members        []CppHookMember
	Written        bool
}

// CppHookMember is one stored field or fn (CODEGEN.md §5.14): Member is passed by value and moved into place, and DefineMember for a ref into a define table; a Resolved slot's entry pointer, `<f>_ref_`, the hook fills instead.
type CppHookMember struct {
	Field        *Field
	Fn           *ExportFn
	Member       string
	Resolved     bool
	DefineMember string
}

// MakeStruct is detail::<P>Make of package pkg, P the UpperCamel of its last segment (CODEGEN.md §3.3, §5.14).
func (pl *CppNamePlan) MakeStruct(pkg string) string { return cppPkgUpper(pkg) + hookMake }

// cppPkgUpper is P of package pkg, the UpperCamel of its last segment.
func cppPkgUpper(pkg string) string { return cppUpperCamel(pkg[strings.LastIndex(pkg, qnameSep)+1:]) }

// MakeName is a record's hook, the member T of <P>Make, of any package.
func (pl *CppNamePlan) MakeName(rec *Record) string { return pl.TypeName(rec) }

// MakeCaseName is a case's hook, V_<Case>, <Case> the name its As<Case> uses (CODEGEN.md §5.5, §5.14).
func (pl *CppNamePlan) MakeCaseName(v *Variant, c *Case) string {
	return pl.TypeName(v) + underscore + strings.TrimPrefix(pl.AsName(c), cppAsPrefix)
}

// MakeCaseTypeName is a case's case-type hook, Case_V_<Case>.
func (pl *CppNamePlan) MakeCaseTypeName(v *Variant, c *Case) string {
	return hookCase + underscore + pl.MakeCaseName(v, c)
}

// MakeBranchName is a dependent type's hook of branch b, D_<Branch>, <Branch> the name its As<Branch> uses (§5.6).
func (pl *CppNamePlan) MakeBranchName(d *Dependent, b *Branch) string {
	return pl.TypeName(d) + underscore + cppUpperCamel(b.Name)
}

// RecordHook is rec's hook, of this package or another; it never takes id_ or retired_ (CODEGEN.md §5.14; log-2026-10-06).
func (pl *CppNamePlan) RecordHook(rec *Record) CppHook {
	return pl.written(CppHook{Name: pl.MakeName(rec), Members: pl.hookMembers(rec, rec.Pkg)}, rec.Pkg)
}

// written sets Written: not for a class of this package that a non-baked loader resolves a ref of.
func (pl *CppNamePlan) written(h CppHook, pkg string) CppHook {
	h.Written = true
	if pkg != pl.p.Name || pl.baked() {
		return h
	}
	for _, m := range h.Members {
		if m.Resolved {
			h.Written = false
		}
	}
	return h
}

// CaseHook is the hook of case c of v, of this package or another, with its case-type hook when c has fields.
func (pl *CppNamePlan) CaseHook(v *Variant, c *Case) CppHook {
	h := CppHook{Name: pl.MakeCaseName(v, c), Members: pl.hookMembers(c, v.Pkg)}
	if len(c.Fields) > 0 {
		h.CaseType = pl.MakeCaseTypeName(v, c)
	}
	return pl.written(h, v.Pkg)
}

// hookMembers are the stored fields (not an input, not Never?) then the stored fns of a class of package pkg, resolved as pkg lays them out.
func (pl *CppNamePlan) hookMembers(class any, pkg string) []CppHookMember {
	fields, fns := classBody(class)
	var out []CppHookMember
	for _, f := range fields {
		if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
			continue
		}
		m := CppHookMember{Field: f, Member: pl.Member(f.Name), Resolved: pl.ownerResolves(f.Type, class, pkg)}
		if _, member, ok := pl.DefineValue(f); ok {
			m.DefineMember = member
		}
		out = append(out, m)
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			t, _ := goUnwrap(fn.Result)
			out = append(out, CppHookMember{Fn: fn, Member: pl.Member(fn.Name), Resolved: pl.ownerResolves(t, class, pkg)})
		}
	}
	return out
}

// ownerResolves is Resolves for a class of package pkg: this package's own, or a ref another package's hook fills (CODEGEN.md §5.8, §5.14).
func (pl *CppNamePlan) ownerResolves(t TypeRef, class any, pkg string) bool {
	if pkg == pl.p.Name {
		return pl.Resolves(t, class)
	}
	if t.Kind == types.List && t.Elem != nil {
		t = *t.Elem
	}
	return t.Kind == types.Ref && ownerResolves(pl.p, pkg, TargetCpp, t.Ref)
}

// declareMake declares detail::<P>Make and its members, each hook of the package: a member is named after the type it builds, which it writes fully qualified, so only the struct's own name is hidden (CODEGEN.md §3.5, §5.14).
func (pl *CppNamePlan) declareMake(detail *nameScope) {
	name := pl.MakeStruct(pl.p.Name)
	pl.shareInner(detail, name, pl.p.Name, nil)
	sc := pl.scope(name)
	sc.hidden = map[string]string{name: pl.p.Name}
	pl.declareHooks(sc, pl.p, pl) // then entry, then row hooks: hook vs hook E8005 follows this order (log-2026-10-06 "U1 re-review")
	for _, t := range pl.p.Types {
		if rec, ok := t.(*Record); ok && pl.entry(rec) {
			pl.declareHook(sc, rec.QName(), rec, derivation{rec, func() string { return pl.EntryHook(rec) }})
		}
	}
	for _, row := range pl.rows {
		pl.declareHook(sc, row.Origin, rowHolder(row), derivation{row.Record, func() string { return pl.RowHook(row.Record) }})
	}
}

// RowHook is the member of detail::<P>Make building this package's row class of another package's record from the record, its id and its retired flag: <Element>Row (CODEGEN.md §5.9, §5.14).
func (pl *CppNamePlan) RowHook(rec *Record) string { return pl.RowName(rec) }

// EntryHook is the member of detail::<P>Make building an entry of one of P's tables from the record, its id and its retired flag, Entry_<T>, of any package (log-2026-10-06 "U1 review" 1).
func (pl *CppNamePlan) EntryHook(rec *Record) string {
	return hookEntry + underscore + pl.TypeName(rec)
}

// entry reports a record of this package whose class has id_ and retired_: a row of a table value the emit selects or of a table field (CODEGEN.md §5.3, declareRecordOwn).
func (pl *CppNamePlan) entry(rec *Record) bool {
	return pl.NestedRow(rec) || slices.ContainsFunc(pl.values, func(v *Value) bool { return tableRecord(v) == rec })
}
