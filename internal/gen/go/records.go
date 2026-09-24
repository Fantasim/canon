package gogen

import (
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// body is what a record or case type holds: slots, finite methods, a table's id type (§5.3).
type body struct {
	owner   string // Canon name, for messages
	goName  string
	slots   []*slot
	finite  []*finiteMethod
	idType  string // "" unless a table holds the record
	members *scope // storage and methods: Go selectors of one struct share a namespace
}

// recordBody gathers the fields and export fns of a record or case (CODEGEN.md §5.4).
func (g *gen) recordBody(owner, goName string, fields []*ir.Field, fns []*ir.ExportFn) *body {
	b := &body{owner: owner, goName: goName, members: newScope(owner)}
	for _, f := range fields {
		if f.Input != nil {
			g.failf(ErrUnsupported, "input field %s.%s", owner, f.Name)
		}
		if f.Optional && f.Type.Kind == types.Never {
			continue // CODEGEN.md §4.4: a Never? field is not emitted at all
		}
		s := g.newSlot(owner+dot+f.Name, exportedName(f.Go.Name, f.Name), storageName(f.Name), f.Type, f.Optional)
		s.doc, s.field = f.Doc, f.Name
		b.slots = append(b.slots, s)
	}
	for _, fn := range fns {
		g.addMethod(b, owner, fn)
	}
	return b
}

func (g *gen) addMethod(b *body, owner string, fn *ir.ExportFn) {
	origin := owner + dot + fn.Name
	name, store := exportedName(fn.Go.Name, fn.Name), storageName(fn.Name)
	switch fn.Kind {
	case ir.FnPrecomputed:
		t, opt := unwrapOptional(fn.Result)
		s := g.newSlot(origin, name, store, t, opt)
		s.doc, s.fn = fn.Doc, fn
		b.slots = append(b.slots, s)
	case ir.FnLookup:
		b.finite = append(b.finite, g.newFinite(origin, name, store, fn))
	default:
		g.failf(ErrUnsupported, translatedFormat, origin)
	}
}

// typeDecl writes the struct and its getters; names go to the struct's and method scopes.
func (g *gen) typeDecl(b *body, doc string) {
	var members []member
	if b.idType != "" {
		members = append(members, member{idStore, b.idType, b.owner}, member{retiredStore, goBool, b.owner})
	}
	for _, s := range b.slots {
		members = append(members, g.storage(s)...)
	}
	for _, f := range b.finite {
		members = append(members, member{f.store, g.storageType(f), f.origin})
	}
	g.body.WriteString(docFor(b.goName, doc))
	g.printf(structOpen, b.goName)
	for _, m := range members {
		g.fail(b.members.add(m.name, m.origin))
		g.printf("%s %s\n", m.name, m.typ)
	}
	g.printf("}\n\n")
	recv := methodRecv(b.goName)
	if b.idType != "" {
		g.writeGetter(b.members, recv, getter{name: idSuffixUpper, result: b.idType, body: returnKw + selfDot + idStore, origin: b.owner})
		g.writeGetter(b.members, recv, getter{name: retiredMethod, result: goBool, body: returnKw + selfDot + retiredStore, origin: b.owner})
	}
	for _, s := range b.slots {
		for _, gt := range g.getters(s, selfRecv) {
			g.writeGetter(b.members, recv, gt)
		}
	}
	for _, f := range b.finite {
		g.writeFinite(b.members, recv, f)
	}
}

func methodRecv(goName string) string { return "func (self *" + goName + ") " }

// writeGetter writes one getter; a one-statement body stays on one line.
func (g *gen) writeGetter(sc *scope, prefix string, gt getter) {
	g.fail(sc.add(gt.name, gt.origin))
	g.body.WriteString(docFor(gt.name, gt.doc))
	if strings.Contains(gt.body, newline) {
		g.printf("%s%s() %s {\n%s\n}\n\n", prefix, gt.name, gt.result, gt.body)
		return
	}
	g.printf("%s%s() %s { %s }\n\n", prefix, gt.name, gt.result, gt.body)
}

// record emits a record type (CODEGEN.md §5.4).
func (g *gen) record(r *ir.Record) {
	name := g.goName(r)
	g.declare(name, r.QName())
	b := g.recordBody(r.QName(), name, r.Fields, r.Methods)
	if tv := g.tableOf[r]; tv != nil {
		b.idType = idTypeName(name)
	}
	g.typeDecl(b, r.Doc)
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
	return ampersand + g.recordLit(rec, r)
}

// recordLit is R{…}: the id of a table entry, then every slot and finite method.
func (g *gen) recordLit(rec *ir.Record, r *value.Record) string {
	if rec.Pkg != g.p.Name {
		g.failf(ErrUnsupported, "a baked value of %s, a record of another package", rec.QName())
	}
	var parts []pair
	if tv := g.tableOf[rec]; tv != nil && r.Ident != nil {
		parts = append(parts, pair{idStore, idMemberName(idTypeName(g.goName(rec)), r.Ident.Key.S)})
		if r.Ident.Retired {
			parts = append(parts, pair{retiredStore, trueLit})
		}
	}
	b := g.recordBody(rec.QName(), g.goName(rec), rec.Fields, rec.Methods)
	parts = append(parts, g.bodyLit(b, r)...)
	return compositeLit(g.typeName(rec), parts)
}

// bodyLit is the storage of a record or case value: its fields, then its export fns.
func (g *gen) bodyLit(b *body, r *value.Record) []pair {
	var parts []pair
	for _, s := range b.slots {
		var v value.Value
		if s.fn == nil {
			v = g.fieldValue(r, s.field)
		} else {
			v = g.instanceOf(s.fn, s.origin, r).Result
		}
		parts = append(parts, g.assign(s, v)...)
	}
	for _, f := range b.finite {
		inst := g.instanceOf(f.fn, f.origin, r)
		parts = append(parts, pair{f.store, g.cellArray(f, inst.Table)})
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
