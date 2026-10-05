package ir

import "github.com/fantasim/canonlang/internal/types"

// GoHook is a make hook (CODEGEN.md §5.14): Name builds the value (Make_<T>, Make_<V>_<Case>), CaseType a case with fields' case type alone (MakeCase_<V>_<Case>, "" else); Slots are the stored fields and fns in getter order, as the owner lays them out. Written is false for a class of this package whose data loader resolves one of its refs: its names stay reserved, no hook is written (log-2026-10-06 "U1 review" 3); another package's class reads true, its owner's refusal being stage E's.
type GoHook struct {
	Name, CaseType string
	Slots          []GoHookSlot
	Written        bool
}

// GoHookSlot is one stored field or fn a hook takes (CODEGEN.md §5.14): a resolved getter gives no parameter, its key, presence and define value do; a lookup method gives its cells, Finite.
type GoHookSlot struct {
	Field  *Field
	Fn     *ExportFn
	Slot   GoSlot
	Finite *GoFinite
}

// MakeName is a record's make hook, Make_<T>, of any package (CODEGEN.md §5.14).
func (pl *GoNamePlan) MakeName(rec *Record) string { return goHookPrefix + pl.TypeName(rec) }

// MakeCaseName is a case's make hook, Make_<V>_<Case>, <Case> the name its As<Case> uses (§5.5, §5.14).
func (pl *GoNamePlan) MakeCaseName(v *Variant, c *Case) string {
	return goHookPrefix + pl.TypeName(v) + underscore + goExported(c.Go, c.Name)
}

// MakeCaseTypeName is a case's case-type hook, MakeCase_<V>_<Case> (CODEGEN.md §5.14).
func (pl *GoNamePlan) MakeCaseTypeName(v *Variant, c *Case) string {
	return hookMake + hookCase + underscore + pl.TypeName(v) + underscore + goExported(c.Go, c.Name)
}

// MakeBranchName is a dependent type's hook of branch b, Make_<D>_<Branch>; it takes what As<Branch> returns, and As<Branch>Value's for a define branch (§5.6, §5.14).
func (pl *GoNamePlan) MakeBranchName(d *Dependent, b *Branch) string {
	return goHookPrefix + pl.TypeName(d) + underscore + goUpperCamel(b.Name)
}

// RecordHook is rec's hook, of this package or another; it never takes an id or a retired flag: a table's rows fill them (CODEGEN.md §5.14; log-2026-10-06).
func (pl *GoNamePlan) RecordHook(rec *Record) GoHook {
	return pl.written(GoHook{Name: pl.MakeName(rec), Slots: pl.hookSlots(rec.Fields, rec.Methods, rec.Pkg)}, rec.Pkg)
}

// written sets Written: not for a class of this package that a data loader resolves a ref of.
func (pl *GoNamePlan) written(h GoHook, pkg string) GoHook {
	h.Written = true
	if pkg != pl.p.Name || pl.data == nil {
		return h
	}
	for _, s := range h.Slots {
		if s.Slot.Resolved {
			h.Written = false
		}
	}
	return h
}

// CaseHook is the hook of case c of v, of this package or another, with its case-type hook when c has fields.
func (pl *GoNamePlan) CaseHook(v *Variant, c *Case) GoHook {
	h := GoHook{Name: pl.MakeCaseName(v, c), Slots: pl.hookSlots(c.Fields, c.Methods, v.Pkg)}
	if len(c.Fields) > 0 {
		h.CaseType = pl.MakeCaseTypeName(v, c)
	}
	return pl.written(h, v.Pkg)
}

// hookSlots are the stored fields (not an input, not Never?) then the stored fns of a class of package pkg.
func (pl *GoNamePlan) hookSlots(fields []*Field, fns []*ExportFn, pkg string) []GoHookSlot {
	var out []GoHookSlot
	for _, f := range fields {
		if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) {
			out = append(out, GoHookSlot{Field: f, Slot: pl.ownerSlot(f, pkg)})
		}
	}
	for _, fn := range fns {
		switch fn.Kind {
		case FnPrecomputed:
			out = append(out, GoHookSlot{Fn: fn, Slot: pl.ownerMethodSlot(fn, pkg)})
		case FnLookup:
			f := pl.ownerFinite(fn, pkg)
			out = append(out, GoHookSlot{Fn: fn, Slot: f.Result, Finite: &f})
		case FnTranslated: // computed on call, never stored
		}
	}
	return out
}

// ownerSlot is a field's slot as its package pkg lays it out: this package's own (Slot), else resolved only where pkg's hook fills the entry (ownerResolves) and a key with its presence otherwise (CODEGEN.md §5.8, §5.14).
func (pl *GoNamePlan) ownerSlot(f *Field, pkg string) GoSlot {
	if pkg == pl.p.Name {
		return pl.Slot(f)
	}
	s := pl.foreignSlot(f.Type, f.Optional, goExported(f.Go, f.Name), goEffectiveStore(f.Go, f.Name), pkg)
	defineSlot(&s, f)
	return s
}

// ownerMethodSlot is a precomputed method's slot as its package pkg lays it out.
func (pl *GoNamePlan) ownerMethodSlot(fn *ExportFn, pkg string) GoSlot {
	if pkg == pl.p.Name {
		return pl.MethodSlot(fn)
	}
	t, opt := goUnwrap(fn.Result)
	return pl.foreignSlot(t, opt, goExported(fn.Go, fn.Name), goEffectiveStore(fn.Go, fn.Name), pkg)
}

// ownerFinite is a lookup method's table as its package pkg lays it out.
func (pl *GoNamePlan) ownerFinite(fn *ExportFn, pkg string) GoFinite {
	f := pl.Finite(fn)
	if pkg != pl.p.Name {
		t, opt := goUnwrap(fn.Result)
		f.Result = pl.foreignSlot(t, opt, f.Name, f.Store, pkg)
	}
	return f
}

// foreignSlot lays a slot out as package pkg's go emit does: a data loader's layout (a key always) in data mode, else baked's with pkg's own resolution.
func (pl *GoNamePlan) foreignSlot(t TypeRef, opt bool, getter, store, pkg string) GoSlot {
	s := slotWith(t, opt, getter, store, func(r *RefTarget) bool { return ownerResolves(pl.p, pkg, TargetGo, r) })
	if e := ownerEmit(pl.p, pkg, TargetGo); e != nil && e.Mode == ModeData && s.Ref != nil {
		s.Resolved, s.Main, s.Key, s.OK = false, false, true, s.Optional
	}
	return s
}

// EntryHook is the entry hook of a record that is the element of one of its package's tables, MakeEntry_<T>(record, id, retired), of any package (log-2026-10-06 "U1 review" 1).
func (pl *GoNamePlan) EntryHook(rec *Record) string {
	return hookMake + hookEntry + underscore + pl.TypeName(rec)
}

// declareEntryHooks declares the entry hook of each record of the package that a table value or table field holds (isTableRecord).
func (pl *GoNamePlan) declareEntryHooks(top *nameScope) {
	for _, t := range pl.p.Types {
		if rec, ok := t.(*Record); ok && pl.isTableRecord(rec) {
			pl.declareHook(top, rec.QName(), rec, derivation{rec, func() string { return pl.EntryHook(rec) }})
		}
	}
}

// RowHook is the make hook of this package's row type of another package's record, Make_<Element>Row(record, id, retired) (CODEGEN.md §5.9, §5.14).
func (pl *GoNamePlan) RowHook(rec *Record) string { return goHookPrefix + pl.RowName(rec) }

// declareRowHooks declares the hook of each row type, after the types' hooks.
func (pl *GoNamePlan) declareRowHooks(top *nameScope) {
	for _, row := range pl.rows {
		pl.declareHook(top, row.Origin, rowHolder(row), derivation{row.Record, func() string { return pl.RowHook(row.Record) }})
	}
}
