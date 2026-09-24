package gogen

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// leaf is one wire value: its type after the optional, and its field's unit and encoding.
type leaf struct {
	t    ir.TypeRef
	unit types.Unit
	enc  types.Enc
}

// wireType is the Go type encoding/json reads a leaf into; what has no reader yet is refused (CODEGEN.md §6.1, WIRE.md §5).
func (g *gen) wireType(l leaf) string {
	switch l.t.Kind {
	case types.Bool:
		if l.enc == types.EncInt {
			return goInt64
		}
		return goBool
	case types.Int, types.Float:
		return g.goType(l.t)
	case types.String, types.LitUnion:
		g.stringWire(l.t)
		return goString
	case types.Duration:
		return goInt64
	case types.Enum:
		return g.enumWire(l.t)
	case types.Ref:
		return g.refWire(l.t)
	case types.Record, types.Variant:
		g.ownClass(l.t)
		return g.rawType()
	case types.List:
		return g.listWire(l)
	default:
		g.failKind(l.t.Kind)
		return goString
	}
}

func (g *gen) rawType() string { return g.use(jsonPath, jsonPkg) + rawMessage }

// stringWire refuses a literal union whose other arm is not written as a string (WIRE.md §5.9).
func (g *gen) stringWire(t ir.TypeRef) {
	if t.Kind != types.LitUnion || t.Elem == nil || t.Elem.Kind == types.String {
		return
	}
	if e, ok := t.Elem.Named.(*ir.Enum); ok && !e.JSONCodes {
		return
	}
	g.fail(newDetail(ErrUnsupported, g.at, unionFormat, g.at))
}

// enumWire is a member's wire string, or its code with @json(codes) (WIRE.md §5.3).
func (g *gen) enumWire(t ir.TypeRef) string {
	e, ok := t.Named.(*ir.Enum)
	if ok && e.JSONCodes && e.Codes != nil {
		return g.goType(*e.Codes)
	}
	return goString
}

// refWire is a ref's key as the file writes it: a table key is a string (WIRE.md §5.9).
func (g *gen) refWire(t ir.TypeRef) string {
	g.keyType(t)
	if isTableRef(t.Ref) || t.Key == nil {
		return goString
	}
	return g.wireType(leaf{t: *t.Key})
}

// ownClass refuses a record or variant of another package: its decoder is unexported there.
func (g *gen) ownClass(t ir.TypeRef) {
	if pkg := typePkg(t.Named); pkg != g.p.Name {
		g.fail(newDetail(ErrUnsupported, g.at, foreignClassFormat, qname(t.Named), g.at))
	}
}

// listWire reads elements through pointers, so a null element is seen: records and variants stay raw.
func (g *gen) listWire(l leaf) string {
	if l.enc == types.EncBits {
		return goUint64
	}
	elem := g.sub(l.t.Elem)
	w := g.wireType(leaf{t: elem, unit: l.unit, enc: l.enc})
	if isRaw(elem) {
		return sliceOf + w
	}
	return sliceOf + pointer + w
}

// isRaw reports a type read as raw JSON: a record or variant, whose decoder refuses null.
func isRaw(t ir.TypeRef) bool { return t.Kind == types.Record || t.Kind == types.Variant }

// conv is the storage value of src, an expression of the leaf's wire type. Pure conversions
// write nothing to b; the others declare temporaries whose checks return an error at loc.
func (g *gen) conv(b *strings.Builder, l leaf, src string, loc location) string {
	switch l.t.Kind {
	case types.Bool:
		if l.enc == types.EncInt {
			fmt.Fprintf(b, bitCheckFormat, src, g.errAt(loc, notBitText, src))
			return src + isOne
		}
	case types.Duration:
		return g.durationFrom(src, l.unit)
	case types.Enum:
		return g.parsed(b, g.enumParser(l.t), src, loc)
	case types.Ref:
		return g.refKey(b, l.t, src, loc)
	case types.Record, types.Variant:
		return g.decodeInto(b, l.t, src, loc)
	case types.List:
		return g.listFrom(b, l, src, loc)
	default:
	}
	return src
}

// durationFrom converts a count of the field's unit (WIRE.md §5.1).
func (g *gen) durationFrom(src string, unit types.Unit) string {
	if ms := unit.Millis(); ms != 1 {
		src += timesMillisecond + strconv.FormatInt(ms, decimal)
	}
	return g.rt() + durationFromMs + src + rparen
}

// enumParser is the enum's <E>FromCode with @json(codes), else Parse<E> (WIRE.md §5.3).
func (g *gen) enumParser(t ir.TypeRef) string {
	e, ok := t.Named.(*ir.Enum)
	if !ok {
		g.failf(ErrMalformed, "an enum type without its enum at %s", g.at)
		return nilLit
	}
	name := g.goName(e)
	if e.JSONCodes && e.Codes != nil {
		return g.qualify(e.Pkg, g.names.FromCodeName(name))
	}
	return g.qualify(e.Pkg, g.names.ParseName(name))
}

// parsed calls a (T, bool) parser; false fails the load at loc.
func (g *gen) parsed(b *strings.Builder, parse, src string, loc location) string {
	v, ok := g.temp(tempValue), g.temp(tempOK)
	fmt.Fprintf(b, parsedFormat, v, ok, parse, src, g.errAt(loc, unknownValueText, src))
	return v
}

// refKey converts a ref's key to its id type, or a keyed list's key field type (CODEGEN.md §5.3, §5.8).
func (g *gen) refKey(b *strings.Builder, t ir.TypeRef, src string, loc location) string {
	if !isTableRef(t.Ref) {
		if t.Key == nil {
			return src
		}
		return g.conv(b, leaf{t: *t.Key}, src, loc)
	}
	id := g.idType(t.Ref.Elem)
	if g.enumIDs(t.Ref.Pkg) {
		return g.parsed(b, g.qualify(t.Ref.Pkg, g.names.ParseName(id)), src, loc)
	}
	return g.qualify(t.Ref.Pkg, id) + lparen + src + rparen
}

// enumIDs reports a package whose go emit gives table ids an enum: baked or embedded (§5.3).
func (g *gen) enumIDs(pkg string) bool {
	if pkg == g.p.Name {
		return !g.isData()
	}
	for _, ref := range g.p.Imports {
		for _, e := range ref.Emits {
			if ref.Name == pkg && e.Target == ir.TargetGo {
				return e.Mode == ir.ModeBaked || e.Mode == ir.ModeEmbedded
			}
		}
	}
	return false
}

// decodeInto decodes a record or variant of this package into a new value.
func (g *gen) decodeInto(b *strings.Builder, t ir.TypeRef, src string, loc location) string {
	v := g.temp(tempValue)
	fmt.Fprintf(b, decodeIntoFormat, v, g.goName(t.Named), g.lc.Err, g.decodeFunc(t.Named), g.lc.Name, g.locExpr(loc.dot()), src)
	return v
}

// listFrom converts each element: an rt.List, an rt.KeyedList, or the members of a bits mask.
func (g *gen) listFrom(b *strings.Builder, l leaf, src string, loc location) string {
	switch {
	case l.enc == types.EncBits:
		return g.bitsFrom(b, l.t, src, loc)
	case l.t.KeyedBy != nil:
		return g.keyedFrom(b, l.t, src, loc)
	}
	elem := g.sub(l.t.Elem)
	v, i, x := g.temp(tempValue), g.temp(tempIndex), g.temp(tempElem)
	fmt.Fprintf(b, listOpenFormat, v, g.goType(elem), src, i, x)
	item := x
	if !isRaw(elem) {
		fmt.Fprintf(b, ifNilReturnFormat, x, g.nullAt(loc.index(i)))
		item = pointer + x
	}
	e := g.conv(b, leaf{t: elem, unit: l.unit, enc: l.enc}, item, loc.index(i))
	fmt.Fprintf(b, listCloseFormat, v, i, e)
	return g.rt() + makeList + v + rparen
}

// keyedFrom decodes a keyed list's rows and collects their keys (CODEGEN.md §4.2).
func (g *gen) keyedFrom(b *strings.Builder, t ir.TypeRef, src string, loc location) string {
	rec, ok := g.sub(t.Elem).Named.(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, "a keyed list of no record at %s", g.at)
		return nilLit
	}
	kf := g.keyField(t)
	v, k, i, x := g.temp(tempValue), g.temp(tempKey), g.temp(tempIndex), g.temp(tempElem)
	fmt.Fprintf(b, keyedFromFormat, v, g.goName(rec), src, k, g.goType(kf.Type), i, x,
		g.lc.Err, g.decodeFunc(rec), g.lc.Name, g.locExpr(loc.index(i).dot()), g.keyMember(rec, kf))
	return g.rt() + makeKeyedList + v + listSep + k + rparen
}

// bitsFrom lists the members whose code is set in the mask, in ascending code order (WIRE.md §5.3).
func (g *gen) bitsFrom(b *strings.Builder, t ir.TypeRef, src string, loc location) string {
	e, ok := g.sub(t.Elem).Named.(*ir.Enum)
	if !ok || e.Codes == nil {
		g.failf(ErrMalformed, "bits on a list of no @codes enum at %s", g.at)
		return nilLit
	}
	g.checkBits(b, e, src, loc)
	members := slices.Clone(e.Members)
	slices.SortStableFunc(members, func(a, b *ir.EnumMember) int { return cmp.Compare(a.Code, b.Code) })
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = g.qualify(e.Pkg, g.names.MemberName(e, m))
	}
	v, m := g.temp(tempValue), g.temp(tempMember)
	fmt.Fprintf(b, bitsFormat, v, g.typeName(e), m, strings.Join(names, listSep), src)
	return g.rt() + makeList + v + rparen
}

// keyMember is the storage of a keyed list's key on its row: a ref key's key member.
func (g *gen) keyMember(rec *ir.Record, kf *ir.Field) string {
	for _, s := range g.bodyOf(rec).slots {
		if s.src == kf && s.Ref != nil {
			return s.KeyStore
		}
	}
	return g.names.Slot(kf).Store
}
