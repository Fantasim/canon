package cppgen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// at is where Build writes its statements: the text and their indentation.
type at struct {
	w     *writer
	depth int
}

// line writes one statement at a's indentation.
func (a at) line(format string, args ...any) { a.w.linef(a.depth, format, args...) }

// in is the block one level inside a.
func (a at) in() at { return at{a.w, a.depth + 1} }

// place is where Build writes a slot: its value or key member, and its resolved entries' member.
type place struct{ key, ref string }

// absent reports a value Build leaves as constructed: an input field's, or none.
func absent(v value.Value) bool {
	_, none := v.(*value.None)
	return v == nil || none
}

// fillSlot writes a field's, a value's or a stored result's value, and a ref's resolved entries when it resolves (CODEGEN.md §5.8, §5.9).
func (g *gen) fillSlot(a at, p place, t ir.TypeRef, optional bool, v value.Value) {
	if absent(v) {
		return
	}
	g.fillValue(a, p.key, t, optional, v)
	if list, isRef := refSlot(t); isRef && g.pl.Resolves(t, nil) {
		g.fillResolved(a, p.ref, t, list, v)
	}
}

// fillValue writes v into lhs: one assignment for a literal, else the members of the record, variant or list it is.
func (g *gen) fillValue(a at, lhs string, t ir.TypeRef, optional bool, v value.Value) {
	switch {
	case absent(v):
	case g.literalType(t):
		a.line(assignFormat, lhs, g.literalOf(t, v))
	case optional:
		a.line(emplaceStmtFormat, lhs)
		g.fillValue(a, fmt.Sprintf(derefFormat, lhs), t, false, v)
	case t.Kind == types.Record:
		g.fillOwnRecord(a, lhs, t, v)
	case t.Kind == types.Variant:
		g.fillVariant(a, lhs, t, v)
	case t.Kind == types.List && t.Elem != nil:
		g.fillList(a, lhs, t, v)
	case t.Kind == types.Map && t.Key != nil && t.Elem != nil:
		g.fillMap(a, lhs, t, v)
	case t.Kind == types.Table && t.Elem != nil:
		g.fillTable(a, lhs, t, v)
	case t.Kind == types.TypeApp:
		g.malformed(dependentElsewhere, g.at) // E8019 DependentType
	default:
		g.refuseKind(t.Kind, g.at, typeRefused)
	}
}

// literalType reports a type whose value is one C++ expression: a scalar, a key, a list or map of them.
func (g *gen) literalType(t ir.TypeRef) bool {
	switch t.Kind {
	case types.Bool, types.Int, types.Float, types.String, types.Duration, types.Enum, types.LitUnion, types.Ref:
		return true
	case types.List:
		return t.KeyedBy == nil && t.Elem != nil && g.literalType(*t.Elem)
	case types.Map:
		return t.Key != nil && t.Elem != nil && g.literalType(*t.Key) && g.literalType(*t.Elem)
	default:
		return false
	}
}

// literalOf is v as one expression; a list carries its type, so that it assigns a std::optional too.
func (g *gen) literalOf(t ir.TypeRef, v value.Value) string {
	if t.Kind == types.List {
		return g.storage(t) + g.literal(t, v)
	}
	return g.element(t, v)
}

// fillOwnRecord writes a record value of this package, whose members only its own access struct reaches (E8019 CrossPackageBakedValue).
func (g *gen) fillOwnRecord(a at, lhs string, t ir.TypeRef, v value.Value) {
	rec, ok := t.Named.(*ir.Record)
	r, isRecord := v.(*value.Record)
	if !ok || !isRecord || rec.Pkg != g.p.Name {
		g.malformed(fmt.Sprintf(valueFormat, v), g.at)
		return
	}
	g.fillRecord(a, lhs, class{rec: rec}, r)
}

// fillRecord writes a record's or case's members: a table row's id and retired flag, each field, then each stored fn's results for this receiver (CODEGEN.md §5.3, §5.4, §5.10).
func (g *gen) fillRecord(a at, lhs string, c class, r *value.Record) {
	fields, fns := c.shape()
	if c.rec != nil && g.entries[c.rec] && g.rowOfTable(c.rec, r) {
		a.line(assignFormat, lhs+memberAccess+idMember, g.pl.IDName(c.rec)+scopeSep+g.pl.IDMember(r.Ident.Key.S))
		if r.Ident.Retired {
			a.line(assignFormat, lhs+memberAccess+retiredMember, strconv.FormatBool(true))
		}
	}
	for _, f := range fields {
		g.fillField(a, lhs, fields, f, r)
	}
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			g.fillStored(a, lhs, fn, r)
		}
	}
}

// rowOfTable reports r as an entry of a table value of rec's that the emit selects.
func (g *gen) rowOfTable(rec *ir.Record, r *value.Record) bool {
	if r.Ident == nil || r.Ident.Coll == nil || r.Ident.Coll.Kind != types.CollLet {
		return false
	}
	v := g.valueNamed(r.Ident.Coll.Name)
	return v != nil && v.Type.Kind == types.Table && v.Type.Elem.Named == rec
}

// fillField writes one field: a boxed optional through its std::unique_ptr, a dependent value in the branch its discriminant selects, a define ref's values beside its keys.
func (g *gen) fillField(a at, lhs string, fields []*ir.Field, f *ir.Field, r *value.Record) {
	if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
		return
	}
	leave := g.enter(g.at + qnameSep + f.Name)
	defer leave()
	v := g.fieldOf(r, f.Name)
	m, err := g.member(f.Name)
	g.fail(err)
	key := lhs + memberAccess + m
	switch {
	case absent(v):
	case g.boxed[f]:
		a.line(makeUniqueFormat, key, g.storage(f.Type))
		g.fillValue(a, fmt.Sprintf(derefFormat, key), f.Type, false, v)
	case ir.HeldApp(f.Type) != nil:
		g.fillDependent(a, key, fields, f, r)
	default:
		g.fillSlot(a, place{key: key, ref: lhs + memberAccess + g.pl.RefMember(f.Name)}, f.Type, f.Optional, v)
		g.fillDefine(a, lhs, f, v)
	}
}

// fillStored writes a stored method's result for this receiver: one value, or a lookup's cells in domain order (CODEGEN.md §5.10).
func (g *gen) fillStored(a at, lhs string, fn *ir.ExportFn, r *value.Record) {
	in := g.instanceOf(fn, r)
	m, err := g.member(fn.Name)
	g.fail(err)
	p := place{key: lhs + memberAccess + m, ref: lhs + memberAccess + g.pl.RefMember(fn.Name)}
	t, optional := resultType(fn.Result)
	if fn.Kind == ir.FnPrecomputed {
		g.fillSlot(a, p, t, optional, in.Result)
		return
	}
	g.fillCells(a, p, fn, in.Table)
}

// fillCells writes a lookup table's cells into the arrays of p (CODEGEN.md §5.10: dense, row-major in domain order).
func (g *gen) fillCells(a at, p place, fn *ir.ExportFn, table *ir.LookupTable) {
	t, optional := resultType(fn.Result)
	cells, ok := g.tableCells(fn, table)
	if !ok {
		return
	}
	for i, cell := range cells {
		at := strconv.Itoa(i)
		g.fillSlot(a, place{key: fmt.Sprintf(indexFormat, p.key, at), ref: fmt.Sprintf(indexFormat, p.ref, at)}, t, optional, cell)
	}
}

// tableCells are a lookup's cells, false unless they fill its domains (CODEGEN.md §5.10).
func (g *gen) tableCells(fn *ir.ExportFn, table *ir.LookupTable) ([]value.Value, bool) {
	if table == nil || len(table.Cells) != cellCount(g.fnDomains(fn)) {
		g.malformed(bakedCells, g.at+qnameSep+fn.Name)
		return nil, false
	}
	return table.Cells, true
}

// fillResolved points a resolved ref at its entries in Data, which allocate gave every container first (CODEGEN.md §5.8).
func (g *gen) fillResolved(a at, lhs string, t ir.TypeRef, list bool, v value.Value) {
	if !list {
		r, ok := v.(*value.Ref)
		if !ok {
			g.malformed(fmt.Sprintf(valueFormat, v), g.at)
			return
		}
		a.line(assignFormat, lhs, g.entryAddr(t, r.Key))
		return
	}
	l, ok := v.(*value.List)
	if !ok {
		g.malformed(fmt.Sprintf(valueFormat, v), g.at)
		return
	}
	items := make([]string, len(l.Elems))
	for i, e := range l.Elems {
		r, isRef := e.(*value.Ref)
		if !isRef {
			g.malformed(fmt.Sprintf(valueFormat, e), g.at)
			return
		}
		items[i] = g.entryAddr(*t.Elem, r.Key)
	}
	storage, _ := g.refStorageOf(t, true, false)
	a.line(assignFormat, lhs, storage+openBrace+strings.Join(items, listSep)+closeBrace)
}

// entryAddr is the address of the entry keyed k in the container a ref of type t resolves into.
func (g *gen) entryAddr(t ir.TypeRef, k value.Key) string {
	target := t.Ref.Value
	i, ok := g.bk.index[target][k]
	if !ok {
		g.malformed(bakedEntryKey, g.at)
	}
	return fmt.Sprintf(entryAddrFormat, dataPrefix+g.pl.DataMember(target), i)
}

// fillVariant writes a variant value: the case's alternative, then its fields (CODEGEN.md §5.5).
func (g *gen) fillVariant(a at, lhs string, t ir.TypeRef, v value.Value) {
	vt, ok := t.Named.(*ir.Variant)
	r, isRecord := v.(*value.Record)
	var ct *types.CaseType
	if isRecord && r.T != nil {
		ct, _ = r.T.Base().(*types.CaseType)
	}
	if !ok || ct == nil || ct.Index < 0 || ct.Index >= len(vt.Cases) || vt.Pkg != g.p.Name {
		g.malformed(fmt.Sprintf(valueFormat, v), g.at)
		return
	}
	cs := vt.Cases[ct.Index]
	if len(cs.Fields) == 0 {
		a.line(emplaceCaseFormat, lhs, ct.Index)
		return
	}
	local := rowVar(a.depth + 1)
	a.line(openBrace)
	a.in().line(caseRefFormat, local, lhs, ct.Index)
	g.fillRecord(a.in(), local, class{variant: vt, cs: cs}, r)
	a.line(closeBrace)
}

// fillList writes a list of records or variants element by element; a keyed list gets its rows and keys first (CODEGEN.md §4.2).
func (g *gen) fillList(a at, lhs string, t ir.TypeRef, v value.Value) {
	l, ok := v.(*value.List)
	if !ok {
		g.malformed(fmt.Sprintf(valueFormat, v), g.at)
		return
	}
	elem := g.storage(*t.Elem)
	at := func(i int) string { return fmt.Sprintf(indexFormat, lhs, strconv.Itoa(i)) }
	if t.KeyedBy != nil {
		g.keyedRows(a, lhs, t, l)
		at = func(i int) string { return fmt.Sprintf(keyedElemFormat, elem, lhs, strconv.Itoa(i)) }
	} else {
		a.line(resizeFormat, lhs, len(l.Elems))
	}
	for i, e := range l.Elems {
		local := rowVar(a.depth + 1)
		a.line(openBrace)
		a.in().line(rowAliasFormat, local, at(i))
		g.fillValue(a.in(), local, *t.Elem, false, e)
		a.line(closeBrace)
	}
}

// fillMap writes a map whose values are records or variants: its entries in insertion order, each value filled, then the FlatMap built from them (CODEGEN.md §4.2).
func (g *gen) fillMap(a at, lhs string, t ir.TypeRef, v value.Value) {
	m, ok := v.(*value.Map)
	if !ok || len(m.Keys) != len(m.Vals) {
		g.malformed(fmt.Sprintf(valueFormat, v), g.at)
		return
	}
	local, storage := rowVar(a.depth+1), g.storage(t)
	a.line(openBrace)
	a.in().line(entriesFormat, storage, local, len(m.Keys))
	for i, k := range m.Keys {
		entry := fmt.Sprintf(indexFormat, local, strconv.Itoa(i))
		a.in().line(assignFormat, entry+entryFirst, g.element(*t.Key, k))
		g.fillValue(a.in(), entry+entrySecond, *t.Elem, false, m.Vals[i])
	}
	a.in().line(assignFormat, lhs, fmt.Sprintf(fromEntriesMoveFormat, storage, local))
	a.line(closeBrace)
}

// fillTable writes a table field: its rows keyed by their ids, a string here, each with its retired flag (CODEGEN.md §4.2, §5.3).
func (g *gen) fillTable(a at, lhs string, t ir.TypeRef, v value.Value) {
	tab, ok := v.(*value.Table)
	if !ok {
		g.malformed(fmt.Sprintf(valueFormat, v), g.at)
		return
	}
	rec := g.nestedRecord(t)
	elem := g.typeName(rec)
	keys := make([]string, len(tab.Entries))
	for i, r := range tab.Entries {
		if r.Ident == nil {
			g.malformed(bakedEntryKey, g.at)
			return
		}
		keys[i] = quote(r.Ident.Key.S)
	}
	a.line(keyedRowsFormat, lhs, cppString, elem, elem, len(keys), strings.Join(keys, listSep))
	for i, r := range tab.Entries {
		local := rowVar(a.depth + 1)
		a.line(openBrace)
		a.in().line(rowAliasFormat, local, fmt.Sprintf(keyedElemFormat, elem, lhs, strconv.Itoa(i)))
		a.in().line(assignFormat, local+memberAccess+idMember, keys[i])
		if r.Ident.Retired {
			a.in().line(assignFormat, local+memberAccess+retiredMember, strconv.FormatBool(true))
		}
		g.fillRecord(a.in(), local, class{rec: rec}, r)
		a.line(closeBrace)
	}
}

// keyedRows gives a keyed list its default rows and their keys (canon::KeyedList::FromRows).
func (g *gen) keyedRows(a at, lhs string, t ir.TypeRef, l *value.List) {
	kf := g.keyField(t)
	if kf == nil {
		return
	}
	keys := make([]string, len(l.Elems))
	for i, e := range l.Elems {
		r, ok := e.(*value.Record)
		if !ok {
			g.malformed(fmt.Sprintf(valueFormat, e), g.at)
			return
		}
		keys[i] = g.element(kf.Type, g.fieldOf(r, kf.Name))
	}
	elem := g.storage(*t.Elem)
	a.line(keyedRowsFormat, lhs, g.storage(kf.Type), elem, elem, len(l.Elems), strings.Join(keys, listSep))
}

// fillDependent writes a dependent value, or each element of a list of them, into the branch the record's own discriminant selects (CODEGEN.md §5.6).
func (g *gen) fillDependent(a at, key string, fields []*ir.Field, f *ir.Field, r *value.Record) {
	v := g.fieldOf(r, f.Name)
	d, br := g.bakedBranch(fields, *ir.HeldApp(f.Type), r)
	if d == nil {
		return
	}
	bt := d.Branches[br].Type
	target := key
	if f.Optional {
		a.line(emplaceStmtFormat, key)
		target = fmt.Sprintf(derefFormat, key)
	}
	one := func(lhs string, e value.Value) {
		a.line(emplaceBranchFormat, lhs, br, g.element(bt, e))
		if ir.DefinesRef(bt) {
			a.line(assignFormat, lhs+memberAccess+g.pl.Dependent(d).DefineValue, g.defineLit(bt, e))
		}
	}
	l, isList := v.(*value.List)
	if !isList {
		one(target, v)
		return
	}
	a.line(resizeFormat, target, len(l.Elems))
	for i, e := range l.Elems {
		one(fmt.Sprintf(indexFormat, target, strconv.Itoa(i)), e)
	}
}

// defineLit is the value of the define a key e of ref type t names, from the package's define table (CODEGEN.md §5.8).
func (g *gen) defineLit(t ir.TypeRef, e value.Value) string {
	d := ir.DefinesOf(g.p, ir.DefineTarget(t))
	r, isRef := e.(*value.Ref)
	i := -1
	if d != nil && isRef {
		i = slices.Index(d.Names, r.Key.S)
	}
	if i < 0 || i >= len(d.Values) {
		g.malformed(defineMissing, g.at)
		return cppInvalid
	}
	return intLit(d.Values[i])
}

// bakedBranch is the dependent type app applies and the branch r's discriminant selects, read down ir.DiscFields' path; nil where stage E refuses (E8019 DependentType, E3801).
func (g *gen) bakedBranch(fields []*ir.Field, app ir.TypeRef, r *value.Record) (*ir.Dependent, int) {
	d, ok := app.Named.(*ir.Dependent)
	path := ir.DiscFields(fields, app)
	if !ok || path == nil {
		g.malformed(dependentBadPath, g.at)
		return nil, 0
	}
	disc := value.Value(r)
	for _, f := range path {
		rec, isRecord := disc.(*value.Record)
		if !isRecord {
			g.malformed(dependentBadPath, g.at)
			return nil, 0
		}
		disc = g.fieldOf(rec, f.Name)
	}
	m := -1
	switch x := disc.(type) {
	case *value.Member:
		m = x.Index
	case *value.Bool:
		m = boolOrdinal(x.V)
	}
	if m < 0 || m >= len(d.ByMember) || d.ByMember[m] < 0 || d.ByMember[m] >= len(d.Branches) {
		g.malformed(dependentNoBranch, g.at)
		return nil, 0
	}
	return d, d.ByMember[m]
}

// boolOrdinal is a Bool's position in its domain: false, then true (CODEGEN.md §5.6, §5.10).
func boolOrdinal(b bool) int {
	if b {
		return 1
	}
	return 0
}

// fillDefine writes a define ref's values beside its keys, from the package's define table (CODEGEN.md §5.8).
func (g *gen) fillDefine(a at, lhs string, f *ir.Field, v value.Value) {
	_, member, ok := g.pl.DefineValue(f)
	if !ok {
		return
	}
	lookup := func(e value.Value) string { return g.defineLit(f.Type, e) }
	l, isList := v.(*value.List)
	if !isList {
		a.line(assignFormat, lhs+memberAccess+member, lookup(v))
		return
	}
	items := make([]string, len(l.Elems))
	for i, e := range l.Elems {
		items[i] = lookup(e)
	}
	a.line(assignFormat, lhs+memberAccess+member, g.storage(defineValueType(f.Type))+openBrace+strings.Join(items, listSep)+closeBrace)
}

// fnCellMembers is a stored package fn's member of Data: its result, or a lookup's cells, a resolved ref's entries instead of its keys (CODEGEN.md §5.10: a package fn has no key getter).
func (g *gen) fnCellMembers(w *writer, fn *ir.ExportFn) {
	t, optional := resultType(fn.Result)
	storage, init := g.memberType(t, optional), g.memberInit(t, optional)
	if list, isRef := refSlot(t); isRef && g.pl.Resolves(t, nil) {
		storage, init = g.refStorageOf(t, list, optional)
	}
	if n := cellCount(g.fnDomains(fn)); n > 0 {
		storage, init = fmt.Sprintf(arrayFormat, storage, n), initBraces
	}
	w.linef(depthTwo, memberFormat, storage, g.pl.DataMember(fn.Name), init)
}

// fnDomains are a lookup's domains, nil for a precomputed fn.
func (g *gen) fnDomains(fn *ir.ExportFn) []domain {
	if fn.Kind != ir.FnLookup {
		return nil
	}
	return g.domains(fn)
}

// fillFnCells writes a stored package fn's result or cells into Data, a resolved ref as its entries.
func (g *gen) fillFnCells(w *writer, fn *ir.ExportFn) {
	m := dataPrefix + g.pl.DataMember(fn.Name)
	t, optional := resultType(fn.Result)
	cell := func(lhs string, v value.Value) {
		list, isRef := refSlot(t)
		switch {
		case absent(v):
		case isRef && g.pl.Resolves(t, nil):
			g.fillResolved(at{w, 1}, lhs, t, list, v)
		default:
			g.fillValue(at{w, 1}, lhs, t, optional, v)
		}
	}
	if fn.Kind == ir.FnPrecomputed {
		cell(m, fn.Value)
		return
	}
	cells, ok := g.tableCells(fn, fn.Table)
	if !ok {
		return
	}
	for i, v := range cells {
		cell(fmt.Sprintf(indexFormat, m, strconv.Itoa(i)), v)
	}
}
