package gogen

import (
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// rowView is the shape of text/hooks.txt's row.
type rowView struct {
	Name, Record, RecordType, ID, IDType, Retired string
	IDGetter, RetiredGetter, RecordGetter         string
}

// rowTypes writes this package's row type of each other package's record its tables hold (CODEGEN.md §5.9): the record in an unexported field, the id and retired flag, then one forwarding method per method of the record.
func (g *gen) rowTypes() {
	for _, fr := range g.names.Rows() {
		g.rowType(fr.Record)
	}
}

func (g *gen) rowType(rec *ir.Record) {
	defer g.enter(rec.QName())()
	row := g.names.Row(rec)
	g.exec(tmplRow, rowView{
		Name: row.Name, Record: rowRecordStore, RecordType: g.typeName(rec), ID: ir.GoIDStore, IDType: row.ID,
		Retired: ir.GoRetiredStore, IDGetter: ir.GoID, RetiredGetter: ir.GoRetired, RecordGetter: rowRecordGetter,
	})
	defer g.withRT(rec.Pkg)()
	recv := methodRecv(row.Name)
	for _, hs := range row.Slots {
		origin := rec.QName() + dot + slotCanon(hs)
		if hs.Finite != nil {
			g.forwardFinite(recv, g.newFiniteFrom(origin, hs.Fn, *hs.Finite))
			continue
		}
		s := g.newSlot(origin, hs.Slot)
		s.doc = slotDoc(hs)
		for _, gt := range g.forwardGetters(s) {
			g.writeGetter(recv, gt)
		}
	}
	for _, fn := range row.Translated {
		g.forwardTranslated(recv, fn)
	}
}

// forwardGetters are the getters of a slot of the held record, as the record declares them (getters), each calling the record's.
func (g *gen) forwardGetters(s *slot) []getter {
	var out []getter
	if s.hasMain() {
		out = append(out, getter{name: s.Getter, result: results(g.mainType(s), s.mainOK()), doc: s.doc})
	}
	if s.Ref != nil {
		key := getter{name: s.KeyGetter, result: results(g.slotKeyType(s), s.Optional)}
		if !s.hasMain() {
			key.doc = s.doc
		}
		out = append(out, key)
	}
	if s.Define {
		out = append(out, getter{name: s.ValueGetter, result: results(g.defineType(s), s.needsOK()), doc: s.doc})
	}
	for i := range out {
		out[i].body = forwardCall(out[i].name, "")
	}
	return out
}

// slotCanon and slotDoc are a hook slot's Canon name and doc: its field's or fn's.
func slotCanon(hs ir.GoHookSlot) string {
	if hs.Field != nil {
		return hs.Field.Name
	}
	return hs.Fn.Name
}

func slotDoc(hs ir.GoHookSlot) string {
	if hs.Field != nil {
		return hs.Field.Doc
	}
	return hs.Fn.Doc
}

// forwardCall is the body of a forwarding method: the held record's method of the same name.
func forwardCall(name, args string) string {
	return returnKw + selfDot + rowRecordStore + dot + name + lparen + args + rparen
}

// forwardFinite forwards a lookup method: its entry getter when it has one, and a ref result's key getter (CODEGEN.md §5.10).
func (g *gen) forwardFinite(recv string, f *finiteMethod) {
	defer g.enter(f.origin)()
	sig := make([]string, len(f.fn.Params))
	for i, p := range f.fn.Params {
		sig[i] = f.params[i] + space + g.paramType(p.Type)
	}
	h := finiteHead{prefix: recv, params: strings.Join(sig, listSep)}
	args := strings.Join(f.params, listSep)
	name, result := f.name, results(g.cellType(f), f.pair)
	if f.res.Ref != nil && !f.res.hasMain() {
		name = f.res.KeyGetter
	}
	g.writeFiniteFunc(h, name, f.fn.Doc, result, forwardCall(name, args))
	if f.res.Ref != nil && f.res.hasMain() {
		key := results(g.slotKeyType(f.res), f.res.Optional)
		g.writeFiniteFunc(h, f.res.KeyGetter, "", key, forwardCall(f.res.KeyGetter, args))
	}
}

// forwardTranslated forwards a translated method with its own signature (CODEGEN.md §5.10).
func (g *gen) forwardTranslated(recv string, fn *ir.ExportFn) {
	sig := make([]string, len(fn.Params))
	args := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		args[i] = g.paramLocal(p.Name)
		sig[i] = args[i] + space + g.goType(p.Type)
	}
	h := finiteHead{prefix: recv, params: strings.Join(sig, listSep)}
	name := g.names.MethodSlot(fn).Getter // the public method's name, UpperCamel(fn) or its override (§3.3)
	g.writeFiniteFunc(h, name, fn.Doc, g.goType(fn.Result), forwardCall(name, strings.Join(args, listSep)))
}

// paramLocal is a forwarded parameter's local: its Canon name, escaped (CODEGEN.md §3.4).
func (g *gen) paramLocal(name string) string {
	for g.reserved(name) {
		name += underscore
	}
	return name
}
