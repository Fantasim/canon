package ir

import "slices"

// GoRow is this package's row type of another package's record (CODEGEN.md §5.9, DECISIONS 323): Name is `<Element>Row`, ID this package's `<Element>ID`; it holds the record in `record` with `id` and `retired`, has ID, Retired and Record, and forwards each method of the record: every getter of Slots (GoSlot.members, a lookup's GoFinite) and each Translated method.
type GoRow struct {
	Name, ID   string
	Slots      []GoHookSlot
	Translated []*ExportFn
}

// GoRTImport is another package's `rt` that this file imports under Name, `<gopkg>rt`: it builds or forwards values of that package holding lists, maps or keyed lists, which its make hooks take as its own rt types (CODEGEN.md §2.8, §5.14).
type GoRTImport struct {
	Pkg, Name, Path string
}

// Foreign is what the emit builds of other packages' types through their make hooks (CODEGEN.md §2.8).
func (pl *GoNamePlan) Foreign() *ForeignUse { return pl.foreign }

// Rows are the other packages' records the package's tables hold, in first-use order (CODEGEN.md §5.9).
func (pl *GoNamePlan) Rows() []ForeignRow { return pl.rows }

// RowName is this package's row type of another package's record, <Element>Row (CODEGEN.md §3.3).
func (pl *GoNamePlan) RowName(rec *Record) string { return pl.TypeName(rec) + rowSuffix }

// Row is rec's row type in this package, rec being another package's record.
func (pl *GoNamePlan) Row(rec *Record) GoRow {
	r := GoRow{Name: pl.RowName(rec), ID: pl.IDTypeName(rec), Slots: pl.hookSlots(rec.Fields, rec.Methods, rec.Pkg)}
	for _, fn := range rec.Methods {
		if fn.Kind == FnTranslated {
			r.Translated = append(r.Translated, fn)
		}
	}
	return r
}

// ReaderName is this package's data-mode reader of a class of another package (CODEGEN.md §2.8): decode_<gopkg>_<T>, T the class's Go name in its package, a case's its case type; the interior _ keeps it apart from decode<T> of this package's classes.
func (pl *GoNamePlan) ReaderName(class any) string {
	pkg := pl.foreign.classPkg(class)
	e := ownerEmit(pl.p, pkg, TargetGo)
	if e == nil {
		return ""
	}
	name := pl.goNameOfAny(class)
	return goDecodePrefix + underscore + e.GoPackage + underscore + name
}

// goNameOfAny is a class's Go type name, of any package: a case's is its case type.
func (pl *GoNamePlan) goNameOfAny(class any) string {
	if c, ok := class.(*Case); ok {
		return pl.CaseName(pl.foreign.VariantOf(c), c)
	}
	t, _ := class.(Type)
	return pl.TypeName(t)
}

// RTImports are the other packages' rt this file imports, in import order.
func (pl *GoNamePlan) RTImports() []GoRTImport {
	need := map[string]bool{}
	mark := func(pkg string, slots []GoHookSlot) {
		need[pkg] = need[pkg] || slices.ContainsFunc(slots, func(s GoHookSlot) bool { return rtKind(s.Slot.T) })
	}
	for _, c := range pl.foreign.Built() {
		if d, ok := c.(*Dependent); ok {
			need[d.Pkg] = need[d.Pkg] || slices.ContainsFunc(d.Branches, func(b *Branch) bool { return rtKind(b.Type) })
			continue
		}
		fields, fns := classBody(c)
		mark(pl.foreign.classPkg(c), pl.hookSlots(fields, fns, pl.foreign.classPkg(c)))
	}
	for _, row := range pl.rows {
		mark(row.Record.Pkg, pl.hookSlots(row.Record.Fields, row.Record.Methods, row.Record.Pkg))
	}
	var out []GoRTImport
	for _, ref := range pl.p.Imports {
		if e := ownerEmit(pl.p, ref.Name, TargetGo); need[ref.Name] && e != nil {
			out = append(out, GoRTImport{Pkg: ref.Name, Name: e.GoPackage + goRT, Path: e.GoImport + pathSep + goRT})
		}
	}
	return out
}

// declareForeignRows declares, after the package's types, each row type with its own scope and each id type only a table field of another package's record has (CODEGEN.md §3.5, §5.3, §5.9).
func (pl *GoNamePlan) declareForeignRows(top *nameScope) {
	for _, row := range pl.rows {
		holder := rowHolder(row)
		if row.Field != nil && !pl.isTableValueRecord(row.Record) {
			pl.declareFrom(top, row.Origin, holder, derivation{row.Record, func() string { return pl.IDTypeName(row.Record) }})
		}
		pl.declareFrom(top, row.Origin, holder, derivation{row.Record, func() string { return pl.RowName(row.Record) }})
		pl.declareRowBody(row, holder)
	}
}

// rowHolder is the item a row's names are reported at: its first holder in this package.
func rowHolder(row ForeignRow) any {
	if row.Value != nil {
		return row.Value
	}
	return row.Field
}

// declareRowBody declares a row's getters, ID, Retired and Record first, then the record's forwarded methods, then its storage: a forwarded method named like one of the row's own is E8005 (CODEGEN.md §5.9).
func (pl *GoNamePlan) declareRowBody(row ForeignRow, holder any) {
	r := pl.Row(row.Record)
	sc := pl.scope(r.Name)
	for _, n := range []string{GoID, GoRetired, goRecord} {
		pl.declare(sc, n, r.Name, holder)
	}
	origin := row.Record.QName() + qnameSep
	for _, s := range r.Slots {
		for _, n := range forwardedNames(s) {
			pl.declare(sc, n, origin+slotName(s), holder)
		}
	}
	for _, fn := range r.Translated {
		pl.declare(sc, goExported(fn.Go, fn.Name), origin+fn.Name, holder)
	}
	for _, n := range []string{goRecordStore, GoIDStore, GoRetiredStore} {
		pl.declare(sc, n, r.Name, holder)
	}
}

// forwardedNames are the methods a hook slot's getters are: a field's or precomputed fn's getters, a lookup's entry getter and key getter (CODEGEN.md §5.8, §5.10).
func forwardedNames(s GoHookSlot) []string {
	if s.Finite == nil {
		_, getters := s.Slot.members()
		return getters
	}
	var out []string
	if s.Finite.Result.Main {
		out = append(out, s.Finite.Name)
	}
	if s.Finite.Result.Ref != nil {
		out = append(out, s.Finite.Result.KeyGetter)
	}
	return out
}

// slotName is the Canon name of a hook slot's field or fn.
func slotName(s GoHookSlot) string {
	if s.Field != nil {
		return s.Field.Name
	}
	return s.Fn.Name
}

// declareReaders declares data mode's reader of each class of another package its loaders read (CODEGEN.md §2.8); two reached packages giving one name are E8005 at this package's import of the later (log-2026-10-06 "U1 review" 4).
func (pl *GoNamePlan) declareReaders(top *nameScope) {
	for _, c := range pl.foreign.Read {
		if name := pl.ReaderName(c); name != "" {
			pl.declare(top, name, pl.foreign.classOrigin(c), pl.importRef(pl.foreign.classPkg(c)))
		}
	}
}

// declareRTImports declares each other package's rt the file imports, `<gopkg>rt` (CODEGEN.md §2.8, §3.4), at the package's import for a collision.
func (pl *GoNamePlan) declareRTImports(sc *nameScope) {
	for _, imp := range pl.RTImports() {
		pl.declare(sc, imp.Name, imp.Path, pl.importRef(imp.Pkg))
	}
}

// importRef is p's import of package pkg, which stage E locates at its import declaration; nil (no item) when p has none.
func (pl *GoNamePlan) importRef(pkg string) any {
	if i := slices.IndexFunc(pl.p.Imports, func(r *PackageRef) bool { return r.Name == pkg }); i >= 0 {
		return pl.p.Imports[i]
	}
	return nil
}
