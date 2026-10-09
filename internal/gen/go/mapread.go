package gogen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// mapRead is the data of the mapRead template: one map the loader reads in file order.
type mapRead struct {
	Keys, KRaws, Vals, Err, Name, Loc, Raw, KS, VS, KT, VT, I, KP, KCode, VCode, KX, VX string
	Numeric                                                                             bool
}

// mapWalk is the data of the mapWalk template: one map the resolver walks, its key's path in KP.
type mapWalk struct {
	Key, Value, Map, KP, Loc, Text, Check, Body string
}

// readMap reads an object as an rt.Map, its members in file order, each key as the wire key WIRE.md §5.8 gives its type (CODEGEN.md §5.9, DECISIONS 312).
func (g *gen) readMap(b *strings.Builder, l leaf, raw string, loc location) string {
	key, elem := g.sub(l.t.Key), g.sub(l.t.Elem)
	g.called[helperMap] = true
	m := mapRead{
		Keys: g.temp(tempKey), KRaws: g.temp(tempRaw), Vals: g.temp(tempValue), Err: g.lc.Err, Name: g.lc.Name,
		Loc: g.locExpr(loc.dot()), Raw: raw, KS: g.temp(tempKey), VS: g.temp(tempValue), KT: g.goType(key), VT: g.goType(elem),
		I: g.temp(tempIndex), KP: g.temp(tempPath), Numeric: ir.NumericMapKey(key),
	}
	at := heldAt(m.KP)
	var k, v strings.Builder
	m.KX = g.readValue(&k, leaf{t: key}, m.KRaws+lbracket+m.I+rbracket, at)
	m.VX = g.readValue(&v, leaf{t: elem, unit: l.unit, enc: l.enc, disc: l.disc}, m.Vals+lbracket+m.I+rbracket, at)
	m.KCode, m.VCode = k.String(), v.String()
	g.execTo(b, tmplMapRead, m)
	return g.rt() + makeMap + m.KS + listSep + m.VS + rparen
}

// keyTextExpr is the text of the map key expr of type t as the file wrote it (WIRE.md §5.8): its string, an integer's decimal, an enum's wire value or, with @json(codes), its code, a ref's key.
func (g *gen) keyTextExpr(t ir.TypeRef, expr string) string {
	switch t.Kind {
	case types.Int:
		return fmt.Sprintf(formatIntFormat, g.use(strconvPkg, strconvPkg), goInt64+lparen+expr+rparen)
	case types.Enum:
		return g.enumKeyText(t, expr)
	case types.Ref:
		return g.refKeyText(t, expr)
	default:
		return expr
	}
}

// enumKeyText is an enum key's file text: its code in decimal under @json(codes), else its wire value.
func (g *gen) enumKeyText(t ir.TypeRef, expr string) string {
	if e, ok := t.Named.(*ir.Enum); ok && ir.NumericMapKey(t) {
		return fmt.Sprintf(formatIntFormat, g.use(strconvPkg, strconvPkg), g.codeInt64(e, expr))
	}
	return expr + dot + ir.GoWire + callSuffix
}

// refKeyText is a ref key's file text, mirroring keyType's cases (gotype.go): an id enum's wire value, a string id, else its key type's.
func (g *gen) refKeyText(t ir.TypeRef, expr string) string {
	if isTableRef(t.Ref) && g.enumIDs(t.Ref.Pkg) {
		return expr + dot + ir.GoString + callSuffix
	}
	if t.Key != nil && !isTableRef(t.Ref) && !isFieldRef(t.Ref) {
		return g.keyTextExpr(*t.Key, expr)
	}
	return goString + lparen + expr + rparen
}

// walkMap resolves the classes a map's values hold and checks its ref keys name an entry, each member in turn, on the path its key gives (WIRE.md §5.8, §5.9).
func (g *gen) walkMap(b *strings.Builder, t ir.TypeRef, expr string, loc location) {
	elem, k := g.temp(tempElem), g.temp(tempKey)
	w := mapWalk{Key: k, Map: expr, KP: g.temp(tempPath), Loc: g.locExpr(loc.dot()), Text: g.keyTextExpr(g.sub(t.Key), k)}
	var body strings.Builder
	if g.valueWalks(g.sub(t.Elem)) {
		w.Value = listSep + elem
		g.walkValue(&body, g.sub(t.Elem), elem, heldAt(w.KP), false)
	}
	if target := g.names.MapKeyTarget(t, g.walkClass); target != nil {
		w.Check = fmt.Sprintf(keyCheckFormat, g.temp(tempOK), g.findIn(target, g.walkSnap), k, g.errAt(heldAt(w.KP), noEntryText, w.Text))
	}
	w.Body = body.String()
	g.execTo(b, tmplMapWalk, w)
}
