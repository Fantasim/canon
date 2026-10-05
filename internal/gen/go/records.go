package gogen

import (
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// body is what a record or case type holds: slots, finite and translated methods, a table's id type (§5.3).
type body struct {
	key        any    // the *ir.Record or *ir.Case, which data mode's holders index
	owner      string // Canon name, for messages
	canon      string // Potion, or Reward.item for a case: how conformance failures name it
	goName     string
	fields     []*ir.Field
	methods    []*ir.ExportFn
	slots      []*slot
	finite     []*finiteMethod
	translated []*ir.ExportFn
	idType     string     // "" unless a table holds the record
	pkg        string     // the Canon package of the class
	foreign    bool       // another package's class, its slots as its make hook takes them (CODEGEN.md §5.14)
	hook       []hookItem // a foreign class's hook parameters, in order
}

// hookItem is one parameter group of a make hook: a stored field's or precomputed fn's slot, or a lookup's cells.
type hookItem struct {
	s *slot
	f *finiteMethod
}

// recordBody gathers the fields and export fns of a record or case (CODEGEN.md §5.4).
func (g *gen) recordBody(key any, owner, goName string, fields []*ir.Field, fns []*ir.ExportFn) *body {
	b := &body{key: key, owner: owner, goName: goName, fields: fields, methods: fns, pkg: g.p.Name}
	for _, f := range fields {
		if f.Optional && f.Type.Kind == types.Never {
			continue // CODEGEN.md §4.4: a Never? field is not emitted at all
		}
		s := g.newSlot(owner+dot+f.Name, g.names.Slot(f))
		s.doc, s.field, s.src = f.Doc, f.Name, f
		if f.Input != nil {
			r, ok := key.(*ir.Record)
			if !ok {
				g.failf(ErrMalformed, "input field %s.%s on %T, not a record (EVALUATION.md §11.1)", owner, f.Name, key)
			}
			s.rec = r
		}
		b.slots = append(b.slots, s)
	}
	for _, fn := range fns {
		g.addMethod(b, owner, fn)
	}
	return b
}

func (g *gen) addMethod(b *body, owner string, fn *ir.ExportFn) {
	origin := owner + dot + fn.Name
	switch fn.Kind {
	case ir.FnPrecomputed:
		s := g.newSlot(origin, g.names.MethodSlot(fn))
		s.doc, s.fn = fn.Doc, fn
		b.slots = append(b.slots, s)
	case ir.FnLookup:
		f := g.newFinite(origin, fn)
		f.method = true
		g.checkLookupParams(f)
		b.finite = append(b.finite, f)
	default:
		b.translated = append(b.translated, fn)
	}
}

// checkLookupParams refuses a data-mode lookup over a parameter that is no enum or Bool (CODEGEN.md §5.10).
func (g *gen) checkLookupParams(f *finiteMethod) {
	if !g.isData() {
		return
	}
	for _, p := range f.fn.Params {
		if p.Type.Kind != types.Bool && p.Type.Kind != types.Enum {
			g.fail(newDetail(ErrMalformed, f.origin, lookupParamFormat, f.origin)) // E8013 refParam
		}
	}
}

// typeDecl writes the struct and its getters, as the plan declared them.
func (g *gen) typeDecl(b *body, doc string) {
	var members []member
	if b.idType != "" {
		members = append(members, member{ir.GoIDStore, b.idType}, member{ir.GoRetiredStore, goBool})
	}
	for _, s := range b.slots {
		members = append(members, g.storage(s)...)
	}
	for _, f := range b.finite {
		members = append(members, member{f.store, g.storageType(f)})
		if g.isData() && f.res.Resolved {
			members = append(members, member{f.res.KeyStore, g.keyStorageType(f)})
		}
	}
	g.body.WriteString(docFor(b.goName, doc))
	g.printf(structOpen, b.goName)
	for _, m := range members {
		g.printf("%s %s\n", m.name, m.typ)
	}
	g.printf("}\n\n")
	recv := methodRecv(b.goName)
	if b.idType != "" {
		g.writeGetter(recv, getter{name: ir.GoID, result: b.idType, body: returnKw + selfDot + ir.GoIDStore})
		g.writeGetter(recv, getter{name: ir.GoRetired, result: goBool, body: returnKw + selfDot + ir.GoRetiredStore})
	}
	for _, s := range b.slots {
		for _, gt := range g.getters(s, selfRecv) {
			g.writeGetter(recv, gt)
		}
	}
	for _, f := range b.finite {
		g.writeFinite(recv, f)
	}
	for _, fn := range b.translated {
		g.translatedMethod(b, fn)
	}
}

func methodRecv(goName string) string { return "func (self *" + goName + ") " }

// writeGetter writes one getter; a one-statement body stays on one line.
func (g *gen) writeGetter(prefix string, gt getter) {
	g.body.WriteString(docFor(gt.name, gt.doc))
	if strings.Contains(gt.body, newline) {
		g.printf("%s%s() %s {\n%s\n}\n\n", prefix, gt.name, gt.result, gt.body)
		return
	}
	g.printf("%s%s() %s { %s }\n\n", prefix, gt.name, gt.result, gt.body)
}

// record emits a record type (CODEGEN.md §5.4).
func (g *gen) record(r *ir.Record) {
	g.typeDecl(g.bodyOf(r), r.Doc)
}

// bodyOf is the body of a record, built once (data mode reads it again).
func (g *gen) bodyOf(r *ir.Record) *body {
	if b := g.bodies[r]; b != nil {
		return b
	}
	if r.Pkg != g.p.Name {
		return g.foreignRecordBody(r)
	}
	b := g.recordBody(r, r.QName(), g.goName(r), r.Fields, r.Methods)
	b.canon = r.Name
	if g.tableOf[r] != nil || g.names.NestedRow(r) {
		b.idType = g.idType(r)
	}
	g.bodies[r] = b
	return b
}

// caseBody is the body of a variant's case with fields, built once.
func (g *gen) caseBody(v *ir.Variant, c *ir.Case) *body {
	if b := g.bodies[c]; b != nil {
		return b
	}
	if v.Pkg != g.p.Name {
		return g.foreignCaseBody(v, c)
	}
	b := g.recordBody(c, v.QName()+dot+c.Name, g.names.CaseName(v, c), c.Fields, c.Methods)
	b.canon = v.Name + dot + c.Name
	g.bodies[c] = b
	return b
}

// recordExpr is a record value as its getters return it: *R, pointing at the table entry
// when the value is an entry of an emitted table.
func (g *gen) recordExpr(t ir.TypeRef, r *value.Record) string {
	if ptr, ok := g.entryPointer(r); ok {
		return ptr
	}
	rec, ok := t.Named.(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, "a record value of type %s", qname(t.Named))
		return nilLit
	}
	if rec.Pkg != g.p.Name {
		return addressOf(g.typeName(rec), g.recordLit(rec, r))
	}
	return ampersand + g.recordLit(rec, r)
}

// recordLit is R{…}: the id of a table entry, then every slot and finite method; another package's record is a call of its make hook (CODEGEN.md §2.8).
func (g *gen) recordLit(rec *ir.Record, r *value.Record) string {
	if rec.Pkg != g.p.Name {
		return g.hookCall(rec.Pkg, g.names.RecordHook(rec).Name, g.bodyOf(rec), r)
	}
	var parts []pair
	if tv := g.tableOf[rec]; tv != nil && r.Ident != nil {
		parts = append(parts, pair{ir.GoIDStore, g.idMember(rec, r.Ident.Key.S)})
		if r.Ident.Retired {
			parts = append(parts, pair{ir.GoRetiredStore, trueLit})
		}
	}
	parts = append(parts, g.bodyLit(g.bodyOf(rec), r)...)
	return compositeLit(g.typeName(rec), parts)
}

// bodyLit is the storage of a record or case value: its fields, then its export fns.
func (g *gen) bodyLit(b *body, r *value.Record) []pair {
	var parts []pair
	for _, s := range b.slots {
		if s.isInput() {
			continue
		}
		switch {
		case s.fn != nil:
			parts = append(parts, g.assign(s, g.instanceOf(s.fn, s.origin, r).Result)...)
		case ir.HeldApp(s.T) != nil:
			parts = append(parts, g.assignDependent(b, r, s, g.fieldValue(r, s.field))...)
		default:
			parts = append(parts, g.assign(s, g.fieldValue(r, s.field))...)
		}
	}
	for _, f := range b.finite {
		inst := g.instanceOf(f.fn, f.origin, r)
		lit, _ := g.cellArray(f, inst.Table)
		parts = append(parts, pair{f.store, lit})
	}
	return parts
}

// compositeLit is T{…} with one member per line.
func compositeLit(typ string, parts []pair) string {
	if len(parts) == 0 {
		return typ + emptyBraces
	}
	var b strings.Builder
	b.WriteString(typ + lbrace + newline)
	for _, p := range parts {
		b.WriteString(p.name + keyValueSep + p.expr + listEnd)
	}
	return b.String() + rbrace
}

// fieldValue is the value of the field named name, found through the value's declaration.
func (g *gen) fieldValue(r *value.Record, name string) value.Value {
	for i, f := range declFields(r.T) {
		if f.Name == name && i < len(r.Fields) {
			return r.Fields[i]
		}
	}
	g.failf(ErrMalformed, "a record value without field %s", name)
	return nil
}

func declFields(t types.Type) []*types.Field {
	if t == nil {
		return nil
	}
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d.Fields
	case *types.CaseType:
		return d.Fields
	case *types.AppliedRecord:
		return d.Rec.Fields
	}
	return nil
}
