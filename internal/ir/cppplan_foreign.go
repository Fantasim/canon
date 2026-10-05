package ir

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// Foreign is what the emit builds of other packages' types through their make hooks (CODEGEN.md §2.8).
func (pl *CppNamePlan) Foreign() *ForeignUse { return pl.foreign }

// Rows are the other packages' records the package's tables hold, in first-use order (CODEGEN.md §5.9).
func (pl *CppNamePlan) Rows() []ForeignRow { return pl.rows }

// RowName is this package's row class of another package's record, <Element>Row, derived publicly from the record's class (CODEGEN.md §3.3, §5.9, §7.2).
func (pl *CppNamePlan) RowName(rec *Record) string { return pl.TypeName(rec) + rowSuffix }

// ForeignLoader is the free data-mode loader of a value whose record is another package's, Load<V> in this package's namespace; "" for any other value (CODEGEN.md §5.9, §7.6).
func (pl *CppNamePlan) ForeignLoader(v *Value) string {
	rec, ok := v.Type.Named.(*Record)
	if pl.baked() || v.Reload || v.Type.Kind != types.Record || !ok || rec.Pkg == pl.p.Name {
		return ""
	}
	return CppLoad + cppUpperCamel(v.Name)
}

// ReaderName is this package's reader of a class of another package, in its .gen.cpp's detail namespace (CODEGEN.md §2.7, §2.8): Read_<Seg1>_…_<SegN>_<T>, each segment of the class's whole package path in UpperCamel (`game.core` gives Read_Game_Core_LevelRange), T its class (a case's, its case class), so two reached packages never give one name (log-2026-10-06 "Ruling 4 refined").
func (pl *CppNamePlan) ReaderName(class any) string {
	var name string
	if c, ok := class.(*Case); ok {
		name = pl.CaseName(pl.foreign.VariantOf(c), c)
	} else {
		t, _ := class.(Type)
		name = pl.TypeName(t)
	}
	parts := []string{cppReadPrefix}
	for seg := range strings.SplitSeq(pl.foreign.classPkg(class), qnameSep) {
		parts = append(parts, cppUpperCamel(seg))
	}
	return strings.Join(append(parts, name), underscore)
}

// declareForeign declares, in the namespace, each row class with its members and the free loader of each value of another package's record (CODEGEN.md §5.9).
func (pl *CppNamePlan) declareForeign() {
	for _, row := range pl.rows {
		holder := rowHolder(row)
		pl.shareNSFrom(row.Origin, holder, derivation{row.Record, func() string { return pl.RowName(row.Record) }})
		pl.declareRowClass(row, holder)
	}
	for _, v := range pl.values {
		if name := pl.ForeignLoader(v); name != "" {
			pl.shareNS(name, pl.valueOrigin(v), v)
		}
	}
}

// declareRowClass declares a row class's own members, then the record's public getters it inherits: one named like a member of the row (GetId, GetRetired) is E8005 (CODEGEN.md §5.9); the record's own entry getters, when it is a row of its package, are replaced, never inherited names.
func (pl *CppNamePlan) declareRowClass(row ForeignRow, holder any) {
	name := pl.RowName(row.Record)
	sc := pl.scope(name)
	sc.hidden = map[string]string{name: row.Origin}
	if pl.baked() && row.Value != nil { // a table field's rows are keyed by a string, never the id enum
		sc.hidden[pl.IDName(row.Record)] = row.Origin
	}
	for _, n := range cppEntryMembers {
		pl.declare(sc, n, name, holder)
	}
	origin := row.Record.QName() + qnameSep
	for _, f := range row.Record.Fields {
		for _, g := range pl.inheritedGetters(f, row.Record) {
			pl.declare(sc, g, origin+f.Name, holder)
		}
	}
	for _, fn := range row.Record.Methods {
		getter, resolved := pl.StoredGetter(fn)
		t, _ := goUnwrap(fn.Result)
		if fn.Kind == FnTranslated || pl.ownerResolves(t, row.Record, row.Record.Pkg) {
			pl.declare(sc, resolved, origin+fn.Name, holder)
		}
		if fn.Kind != FnTranslated {
			pl.declare(sc, getter, origin+fn.Name, holder)
		}
	}
}

// inheritedGetters are a field's public getters on its class: none for Never?, the input getter, else its resolved getter where its owner resolves it, its getter and its define value getter (CODEGEN.md §5.4, §5.8).
func (pl *CppNamePlan) inheritedGetters(f *Field, rec *Record) []string {
	getter, resolved := pl.FieldGetter(f)
	switch {
	case f.Optional && f.Type.Kind == types.Never:
		return nil
	case f.Input != nil:
		return []string{getter}
	}
	var out []string
	if pl.ownerResolves(f.Type, rec, rec.Pkg) {
		out = append(out, resolved)
	}
	out = append(out, getter)
	if value, _, ok := pl.DefineValue(f); ok {
		out = append(out, value)
	}
	return out
}

// declareReaders declares, in detail, this package's reader of each class of another package its loaders or decoders read (CODEGEN.md §2.7, §2.8): the .gen.cpp declares them, no header, so other packages never meet them.
func (pl *CppNamePlan) declareReaders(detail *nameScope) {
	if pl.baked() {
		return
	}
	for _, c := range pl.foreign.Read {
		pl.declareInner(detail, pl.ReaderName(c), pl.foreign.classOrigin(c), nil)
	}
}
