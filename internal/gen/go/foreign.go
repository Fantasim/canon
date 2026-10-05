package gogen

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// hookView is a slot as a make hook takes it (CODEGEN.md §5.14): a ref gives its key, its presence when optional and its define value, never its entry.
func hookView(s ir.GoSlot) ir.GoSlot {
	if s.Ref != nil {
		s.Resolved, s.Main, s.Key, s.OK = false, false, true, s.Optional
	}
	return s
}

// viewSlot is s as its hook takes it, owned when its package's hook resolves it.
func viewSlot(s *slot) *slot {
	v := *s
	v.GoSlot = hookView(s.GoSlot)
	v.owned = s.Resolved
	return &v
}

// viewFinite is a lookup as its hook takes it: a ref result's cells are keys.
func (g *gen) viewFinite(f *finiteMethod) *finiteMethod {
	v := *f
	v.res = viewSlot(f.res)
	g.setPair(&v)
	v.split = v.pair // values then presence, never the unexported pair (log-2026-10-06 "U2 review FAIL" 1)
	if f.res.Resolved && f.res.hasMain() {
		v.entry = f.name
	}
	return &v
}

// finiteParams are a lookup's hook parameters as v, its hook view, lays them out: its cells, or its values array then its presence array (CODEGEN.md §5.14).
func (g *gen) finiteParams(v *finiteMethod) []member {
	if !v.split {
		return []member{{v.store, g.storageType(v)}}
	}
	return []member{{v.store, g.dims(v) + g.cellType(v)}, {v.res.OKStore, g.dims(v) + goBool}}
}

// entryType is the Go type of an entry of a ref's target: its record, or the target package's row type when its table holds another package's record (CODEGEN.md §5.8, §5.9).
func (g *gen) entryType(r *ir.RefTarget) string {
	if rec, ok := r.Elem.(*ir.Record); ok && r.Coll == types.CollLet && !r.Keyed && rec.Pkg != r.Pkg {
		return g.qualify(r.Pkg, g.names.RowName(rec))
	}
	return g.typeName(r.Elem)
}

// holder is the package whose class is being written: this one, or under withRT another (CODEGEN.md §4.2).
func (g *gen) holder() string {
	if g.rtPkg == "" {
		return g.p.Name
	}
	return g.rtPkg
}

// addressOf is a pointer to a value of type typ that is no composite literal: a make hook's result.
func addressOf(typ, x string) string {
	return ampersand + sliceOf + typ + lbrace + x + rbrace + firstElem
}

// isRow reports another package's record a table of this package holds (CODEGEN.md §5.9).
func (g *gen) isRow(rec *ir.Record) bool {
	return slices.ContainsFunc(g.names.Rows(), func(r ir.ForeignRow) bool { return r.Record == rec })
}

// foreignRecordBody is another package's record as its make hook takes it, with this package's id type when a table here holds it.
func (g *gen) foreignRecordBody(r *ir.Record) *body {
	b := &body{key: r, pkg: r.Pkg, owner: r.QName(), canon: r.Name, goName: g.typeName(r), fields: r.Fields, methods: r.Methods}
	inTable := slices.ContainsFunc(g.names.Foreign().Tables, func(t ir.TableUse) bool { return t.Record == r })
	if g.tableOf[r] != nil || g.isRow(r) || inTable { // its object holds `$id` and `$retired` (log-2026-10-06 "U2 review FAIL" 3)
		b.idType = g.idType(r)
	}
	return g.foreignBody(b, g.names.RecordHook(r).Slots)
}

// foreignCaseBody is another package's case with fields as its make hooks take it.
func (g *gen) foreignCaseBody(v *ir.Variant, c *ir.Case) *body {
	b := &body{key: c, pkg: v.Pkg, owner: v.QName() + dot + c.Name, canon: v.Name + dot + c.Name, fields: c.Fields, methods: c.Methods}
	b.goName = g.qualify(v.Pkg, g.names.CaseName(v, c))
	return g.foreignBody(b, g.names.CaseHook(v, c).Slots)
}

// foreignBody lays out b, a class of another package, in its hook's parameter order, each slot as the hook takes it (CODEGEN.md §5.14).
func (g *gen) foreignBody(b *body, slots []ir.GoHookSlot) *body {
	defer g.withRT(b.pkg)()
	owner := b.owner
	b.foreign = true
	g.bodies[b.key] = b
	for _, hs := range slots {
		if hs.Finite != nil {
			f := g.viewFinite(g.newFiniteFrom(owner+dot+hs.Fn.Name, hs.Fn, *hs.Finite))
			f.method = true
			b.finite = append(b.finite, f)
			b.hook = append(b.hook, hookItem{f: f})
			continue
		}
		s := viewSlot(g.newSlot(owner, hs.Slot))
		if hs.Field != nil {
			s.origin, s.doc, s.field, s.src = owner+dot+hs.Field.Name, hs.Field.Doc, hs.Field.Name, hs.Field
		} else {
			s.origin, s.doc, s.fn = owner+dot+hs.Fn.Name, hs.Fn.Doc, hs.Fn
		}
		b.slots = append(b.slots, s)
		b.hook = append(b.hook, hookItem{s: s})
	}
	return b
}

// hookParams are the parameters of a class's make hook, as b lays them out: each slot's storage, each lookup's cells (CODEGEN.md §5.14).
func (g *gen) hookParams(b *body) []member {
	defer g.withRT(b.pkg)()
	var out []member
	for _, it := range b.hook {
		if it.f != nil {
			out = append(out, g.finiteParams(it.f)...)
			continue
		}
		out = append(out, g.storage(it.s)...)
	}
	return out
}

// hookCall is package pkg's make hook called on the parts of r, laid out by b (CODEGEN.md §2.8).
func (g *gen) hookCall(pkg, hook string, b *body, r *value.Record) string {
	defer g.withRT(pkg)()
	var args []string
	for _, it := range b.hook {
		args = append(args, g.hookArgs(b, it, r)...)
	}
	return g.qualify(pkg, hook) + callArgs(args)
}

// callArgs is (a, b) on one line, or one argument per line when one spans lines.
func callArgs(args []string) string {
	inline := strings.Join(args, listSep)
	if !strings.Contains(inline, newline) {
		return lparen + inline + rparen
	}
	return lparen + newline + strings.Join(args, listEnd) + listEnd + rparen
}

// hookArgs are one hook item's arguments for the value r: a lookup's cells, or a slot's members, zero when none.
func (g *gen) hookArgs(b *body, it hookItem, r *value.Record) []string {
	if f := it.f; f != nil {
		table := g.instanceOf(f.fn, f.origin, r).Table
		if f.split {
			return g.splitCells(f, table)
		}
		lit, _ := g.cellArray(f, table)
		return []string{lit}
	}
	s := it.s
	var parts []pair
	switch {
	case s.fn != nil:
		parts = g.assign(s, g.instanceOf(s.fn, s.origin, r).Result)
	case ir.HeldApp(s.T) != nil:
		parts = g.assignDependent(b, r, s, g.fieldValue(r, s.field))
	default:
		parts = g.assign(s, g.fieldValue(r, s.field))
	}
	ms := g.storage(s)
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = g.hookZero(s, m)
		if j := slices.IndexFunc(parts, func(p pair) bool { return p.name == m.name }); j >= 0 {
			out[i] = parts[j].expr
		}
	}
	return out
}

// hookZero is the zero of a slot's member m: what a hook takes for none.
func (g *gen) hookZero(s *slot, m member) string {
	switch {
	case m.name == s.OKStore || s.Ref == nil && s.T.Kind == types.Bool:
		return strconv.FormatBool(false)
	case s.List && (m.name == s.KeyStore || m.name == s.ValueStore):
		return m.typ + emptyBraces
	case s.Define && m.name == s.ValueStore:
		return zeroLit
	case s.Ref != nil:
		return g.zeroKeyOf(s.refType())
	}
	switch s.T.Kind {
	case types.String, types.LitUnion:
		return emptyString
	case types.Record, types.Variant, types.Case, types.TypeApp:
		return nilLit
	case types.List, types.Map, types.DepMap, types.Table:
		return m.typ + emptyBraces
	default: // a number, a Duration, an enum: an untyped 0
		return zeroLit
	}
}

// zeroKeyOf is the zero of a ref's key: "" for a string id or key, 0 for an id enum or a number.
func (g *gen) zeroKeyOf(t ir.TypeRef) string {
	switch {
	case isTableRef(t.Ref) && g.enumIDs(t.Ref.Pkg):
		return zeroLit
	case isTableRef(t.Ref), isFieldRef(t.Ref):
		return emptyString
	case t.Key != nil && (t.Key.Kind == types.String || t.Key.Kind == types.LitUnion):
		return emptyString
	}
	return zeroLit
}

// foreignVariantExpr is a value of another package's variant, or of its case type, built by its make hooks (CODEGEN.md §2.8, §5.14).
func (g *gen) foreignVariantExpr(t ir.TypeRef, v *ir.Variant, c *ir.Case, r *value.Record) string {
	h := g.names.CaseHook(v, c)
	if t.Kind == types.Case {
		return addressOf(g.qualify(v.Pkg, g.names.CaseName(v, c)), g.hookCall(v.Pkg, h.CaseType, g.caseBody(v, c), r))
	}
	call := g.qualify(v.Pkg, h.Name) + lparen + rparen
	if len(c.Fields) > 0 {
		call = g.hookCall(v.Pkg, h.Name, g.caseBody(v, c), r)
	}
	return addressOf(g.typeName(v), call)
}

// foreignRowLit is a row of rec, another package's record, in a table of package holder: this package's row composite, else holder's row hook (CODEGEN.md §5.9, §5.14).
func (g *gen) foreignRowLit(rec *ir.Record, r *value.Record, id string) string {
	record := g.recordLit(rec, r)
	retired := strconv.FormatBool(r.Ident != nil && r.Ident.Retired)
	if h := g.holder(); h != g.p.Name {
		return g.qualify(h, g.names.RowHook(rec)) + callArgs([]string{record, id, retired})
	}
	parts := []pair{{rowRecordStore, record}, {ir.GoIDStore, id}}
	if retired == trueLit {
		parts = append(parts, pair{ir.GoRetiredStore, trueLit})
	}
	return compositeLit(g.names.RowName(rec), parts)
}

// rowElem is the type a table or keyed list of rec holds in the holder package: its row type when rec is another package's table record (CODEGEN.md §5.9).
func (g *gen) rowElem(rec *ir.Record, table bool) string {
	h := g.holder()
	if table && rec.Pkg != h {
		return g.qualify(h, g.names.RowName(rec))
	}
	return g.typeName(rec)
}
