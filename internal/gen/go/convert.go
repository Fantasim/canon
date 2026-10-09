package gogen

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// leaf is one wire value: its type after the optional, its field's unit, encoding and a dependent type's discriminant expression (CODEGEN.md §5.6).
type leaf struct {
	t     ir.TypeRef
	unit  types.Unit
	enc   types.Enc
	disc  string
	owned bool // another package's ref its hook resolves: an unknown key is `no entry` (CODEGEN.md §2.8)
}

func (g *gen) rawType() string { return g.use(jsonPath, jsonPkg) + rawMessage }

// readValue reads raw, a present JSON value at loc, as the C++ loader does (WIRE.md §5, CODEGEN.md §7.5).
func (g *gen) readValue(b *strings.Builder, l leaf, raw string, loc location) string {
	switch l.t.Kind {
	case types.Bool:
		if l.enc == types.EncInt {
			return g.readInt(b, raw, loc, 0, 1) + isOne
		}
		return g.readPlain(b, goBool, raw, loc)
	case types.Int:
		return g.readSized(b, l.t, raw, loc)
	case types.Float:
		return g.readPlain(b, g.goType(l.t), raw, loc)
	case types.String:
		return g.readPlain(b, goString, raw, loc)
	case types.LitUnion:
		g.stringWire(l.t)
		return g.readUnion(b, l.t, raw, loc)
	case types.Duration:
		if g.isTypes() {
			return g.sourceDuration(b, raw, loc, l.unit)
		}
		limit := durationMaxMs / l.unit.Millis()
		return g.durationFrom(g.readInt(b, raw, loc, -limit, limit), l.unit)
	case types.Enum:
		return g.readEnum(b, l.t, raw, loc)
	case types.Ref:
		return g.readRef(b, l, raw, loc)
	case types.Record, types.Variant:
		return g.decodeInto(b, l.t, raw, loc)
	case types.List:
		return g.readList(b, l, raw, loc)
	case types.Table:
		return g.readNested(b, l.t, raw, loc)
	case types.Map:
		return g.readMap(b, l, raw, loc)
	case types.TypeApp:
		return g.readDependentValue(b, l, raw, loc)
	default:
		g.failKind(l.t.Kind)
		return raw
	}
}

// readPlain reads raw into a new local of type typ: a string, a bool, a float or an array.
func (g *gen) readPlain(b *strings.Builder, typ, raw string, loc location) string {
	v := g.temp(tempValue)
	prefix, key := g.splitLoc(loc)
	fmt.Fprintf(b, readFormat, v, typ, g.lc.Err, g.helper(helperRead), g.lc.Name, prefix, key, raw)
	return v
}

// readPlainInto is readPlain into the local v.
func (g *gen) readPlainInto(v, typ, raw string, loc location) {
	prefix, key := g.splitLoc(loc)
	g.printf(readFormat, v, typ, g.lc.Err, g.helper(helperRead), g.lc.Name, prefix, key, raw)
}

// readInt reads an integer from lo to hi into a new int64 local.
func (g *gen) readInt(b *strings.Builder, raw string, loc location, lo, hi int64) string {
	n := g.temp(tempInt)
	prefix, key := g.splitLoc(loc)
	fmt.Fprintf(b, intFormat, n, g.lc.Err, g.helper(helperInt), g.lc.Name, prefix, key, raw, lo, hi)
	return n
}

// intBounds is an integer type's range: its width's, UInt64's stopping at Int's maximum (TYPES.md §7.2).
func intBounds(t ir.TypeRef) (lo, hi int64) {
	switch {
	case t.Bits >= int64Bits || t.Bits <= 0:
		if t.Signed {
			return math.MinInt64, math.MaxInt64
		}
		return 0, math.MaxInt64
	case t.Signed:
		return -1 << (t.Bits - 1), 1<<(t.Bits-1) - 1
	}
	return 0, 1<<t.Bits - 1
}

// readSized reads an integer within its type's range, converted to its Go type.
func (g *gen) readSized(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	return g.intOf(t, g.readCount(b, t, raw, loc))
}

// readCount reads an integer within t's range into an int64 local.
func (g *gen) readCount(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	lo, hi := intBounds(t)
	return g.readInt(b, raw, loc, lo, hi)
}

// intOf converts the int64 local n to t's Go type.
func (g *gen) intOf(t ir.TypeRef, n string) string {
	if typ := g.goType(t); typ != goInt64 {
		return typ + lparen + n + rparen
	}
	return n
}

// durationFrom converts a count of the field's unit (WIRE.md §5.1).
func (g *gen) durationFrom(src string, unit types.Unit) string {
	if ms := unit.Millis(); ms != 1 {
		src += timesMillisecond + strconv.FormatInt(ms, decimal)
	}
	return g.ownRT() + durationFromMs + src + rparen
}

// readEnum reads a member's wire string, or its code with @json(codes), and parses it (WIRE.md §5.3); an opened enum keeps any string, while a code no member has still fails: there is no wire value to keep (DECISIONS 339).
func (g *gen) readEnum(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	e, ok := t.Named.(*ir.Enum)
	if !ok {
		g.failf(ErrMalformed, noEnumFormat, g.at)
		return raw
	}
	name := g.goName(e)
	if !e.JSONCodes || e.Codes == nil {
		s := g.readPlain(b, goString, raw, loc)
		if ir.OpenEnum(g.p, g.e, e) { // any string, kept as is (DECISIONS 339)
			return g.typeName(e) + lparen + s + rparen
		}
		return g.parsed(b, g.qualify(e.Pkg, g.names.ParseName(name))+lparen+s+rparen, loc, unknownValueText, s)
	}
	n := g.readCount(b, *e.Codes, raw, loc)
	parse := g.qualify(e.Pkg, g.names.FromCodeName(name)) + lparen + g.intOf(*e.Codes, n) + rparen
	return g.parsed(b, parse, loc, unknownCodeText, n)
}

// parsed calls a (T, bool) parser; false fails the load at loc, naming value.
func (g *gen) parsed(b *strings.Builder, call string, loc location, what, value string) string {
	v, ok := g.temp(tempValue), g.temp(tempOK)
	fmt.Fprintf(b, parsedFormat, v, ok, call, g.errAt(loc, what, value))
	return v
}

// readUnion reads a literal union's string; one over an enum must be a literal or a member's wire value, else unknown (WIRE.md §5.9, CODEGEN.md §5.13).
func (g *gen) readUnion(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	v := g.readPlain(b, goString, raw, loc)
	if t.Elem == nil {
		return v
	}
	e, ok := t.Elem.Named.(*ir.Enum)
	if !ok {
		return v
	}
	conds := make([]string, 0, len(t.Literals)+len(e.Members))
	for _, w := range t.Literals {
		conds = append(conds, v+differs+strconv.Quote(w))
	}
	for _, m := range e.Members {
		conds = append(conds, v+differs+strconv.Quote(m.Wire))
	}
	fmt.Fprintf(b, unionCheckFormat, strings.Join(conds, andSep), g.errAt(loc, unknownValueText, v))
	return v
}

// readRef reads a ref's key: a table id, a baked package's id enum, a keyed list's key (CODEGEN.md §5.3, §5.8).
func (g *gen) readRef(b *strings.Builder, l leaf, raw string, loc location) string {
	t := l.t
	key := g.keyType(t)
	if isFieldRef(t.Ref) {
		return g.readPlain(b, key, raw, loc)
	}
	if !isTableRef(t.Ref) {
		if t.Key == nil {
			return raw
		}
		return g.readValue(b, leaf{t: *t.Key}, raw, loc)
	}
	if g.enumIDs(t.Ref.Pkg) {
		text := unknownValueText
		if l.owned || g.holder() != g.p.Name { // any ref of another package's class, key-only ones included (log-2026-10-06 "gen/go re-verify FAIL")
			text = noEntryText
		}
		s := g.readPlain(b, goString, raw, loc)
		return g.parsed(b, g.qualify(t.Ref.Pkg, g.names.ParseName(g.idType(t.Ref.Elem)))+lparen+s+rparen, loc, text, s)
	}
	return g.readPlain(b, key, raw, loc)
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

// stringWire refuses a literal union whose other arm is not written as a string (WIRE.md §5.9).
func (g *gen) stringWire(t ir.TypeRef) {
	if t.Kind != types.LitUnion || t.Elem == nil || t.Elem.Kind == types.String {
		return
	}
	if e, ok := t.Elem.Named.(*ir.Enum); ok && !e.JSONCodes {
		return
	}
	switch {
	case !ir.StringWire(t.Elem):
		g.fail(newDetail(ErrMalformed, g.at, unionMalformedFormat, g.at))
	case t.Elem.Kind == types.TypeApp:
		g.fail(newDetail(errDependentUnion, g.at, dependentUnionFormat, g.at)) // E8019 DependentType
	default:
		g.fail(newDetail(ErrMalformed, g.at, unionFormat, g.at)) // E8019 RefUnion
	}
}

// decodeInto decodes a record or variant into a new value: this package's decoder, or its reader of another package's class (CODEGEN.md §2.8).
func (g *gen) decodeInto(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	v := g.temp(tempValue)
	fmt.Fprintf(b, decodeIntoFormat, v, g.typeName(t.Named), g.lc.Err, g.decodeFunc(t.Named), g.lc.Name, g.locExpr(loc.dot()), raw)
	return v
}

// readList reads an array element by element: an rt.List, an rt.KeyedList, or the members of a bits mask.
func (g *gen) readList(b *strings.Builder, l leaf, raw string, loc location) string {
	if l.enc == types.EncBits {
		return g.readBits(b, l.t, raw, loc)
	}
	array := g.readPlain(b, sliceOf+g.rawType(), raw, loc)
	if l.t.KeyedBy != nil {
		return g.readKeyed(b, l.t, array, loc)
	}
	elem := g.sub(l.t.Elem)
	v, i, x := g.temp(tempValue), g.temp(tempIndex), g.temp(tempElem)
	fmt.Fprintf(b, listOpenFormat, v, g.goType(elem), array, i, x)
	e := g.readValue(b, leaf{t: elem, unit: l.unit, enc: l.enc, disc: l.disc, owned: l.owned}, x, loc.index(i))
	fmt.Fprintf(b, listCloseFormat, v, i, e)
	return g.rt() + makeList + v + rparen
}

// readKeyed decodes a keyed list's rows and collects their keys (CODEGEN.md §4.2).
func (g *gen) readKeyed(b *strings.Builder, t ir.TypeRef, array string, loc location) string {
	rec, ok := g.sub(t.Elem).Named.(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, "a keyed list of no record at %s", g.at)
		return nilLit
	}
	kf := g.keyField(t)
	v, k, i, x := g.temp(tempValue), g.temp(tempKey), g.temp(tempIndex), g.temp(tempElem)
	fmt.Fprintf(b, keyedFromFormat, v, g.typeName(rec), array, k, g.goType(kf.Type), i, x,
		g.lc.Err, g.decodeFunc(rec), g.lc.Name, g.locExpr(loc.index(i).dot()), g.keyMember(rec, kf))
	g.dupCheck(b, k, kf, loc)
	return g.rt() + makeKeyedList + v + listSep + k + rparen
}

// dupCheck refuses two rows sharing a key, naming the first duplicate in row order (WIRE.md §5.7).
func (g *gen) dupCheck(b *strings.Builder, keys string, kf *ir.Field, loc location) {
	at, first, ok := g.temp(localAt), g.temp(localFirst), g.temp(tempOK)
	token := g.keyTokenExpr(kf.Type, keys+lbracket+at+rbracket)
	prefix, key := g.splitLoc(loc.index(at).dot().key(strings.Join(kf.WirePath, dot)))
	fmt.Fprintf(b, dupCheckFormat, at, first, ok, g.ownRT(), keys, g.lc.Name, prefix, key, token, g.locExpr(loc.index(first)))
}

// keyTokenExpr is expr's canonical JSON token for a duplicate-id error message (WIRE.md §7.3).
func (g *gen) keyTokenExpr(t ir.TypeRef, expr string) string {
	switch t.Kind {
	case types.String, types.LitUnion:
		return g.ownRT() + wireTokenCall + goString + lparen + expr + rparen + rparen
	case types.Int:
		return fmt.Sprintf(formatIntFormat, g.use(strconvPkg, strconvPkg), goInt64+lparen+expr+rparen)
	case types.Enum:
		return g.enumKeyToken(t, expr)
	case types.Ref:
		return g.refKeyToken(t, expr)
	default:
		g.failKind(t.Kind)
		return expr
	}
}

// enumKeyToken is an enum key's wire token: a @codes enum's code, else its wire value (WIRE.md §5.3).
func (g *gen) enumKeyToken(t ir.TypeRef, expr string) string {
	e, ok := t.Named.(*ir.Enum)
	if !ok {
		g.failf(ErrMalformed, noEnumFormat, g.at)
		return expr
	}
	if e.JSONCodes && e.Codes != nil {
		return fmt.Sprintf(formatIntFormat, g.use(strconvPkg, strconvPkg), g.codeInt64(e, expr))
	}
	return g.ownRT() + wireTokenCall + expr + dot + ir.GoWire + callSuffix + rparen
}

// refKeyToken is a ref key's wire token, mirroring keyType's cases (gotype.go, WIRE.md §5.8).
func (g *gen) refKeyToken(t ir.TypeRef, expr string) string {
	if t.Ref != nil && t.Ref.Coll == types.CollDefines {
		return g.ownRT() + wireTokenCall + goString + lparen + expr + rparen + rparen
	}
	if isFieldRef(t.Ref) {
		return g.ownRT() + wireTokenCall + goString + lparen + expr + rparen + rparen
	}
	if isTableRef(t.Ref) {
		if g.enumIDs(t.Ref.Pkg) {
			return g.ownRT() + wireTokenCall + expr + dot + ir.GoString + callSuffix + rparen
		}
		return g.ownRT() + wireTokenCall + goString + lparen + expr + rparen + rparen
	}
	if t.Key != nil {
		return g.keyTokenExpr(*t.Key, expr)
	}
	g.failf(ErrMalformed, noKeyType)
	return expr
}

// readBits lists the members a mask sets, in code order; a bit no code holds fails (WIRE.md §5.3).
func (g *gen) readBits(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	e, ok := g.sub(t.Elem).Named.(*ir.Enum)
	if !ok || e.Codes == nil {
		g.failf(ErrMalformed, "bits on a list of no @codes enum at %s", g.at)
		return nilLit
	}
	mask := g.uint64Of(g.readInt(b, raw, loc, 0, math.MaxInt64))
	rest := g.temp(tempValue)
	fmt.Fprintf(b, bitsCheckFormat, rest, mask, g.bitsMask(e), g.errAt(loc, unknownBitsText, rest))
	members := slices.Clone(e.Members)
	slices.SortStableFunc(members, func(a, b *ir.EnumMember) int { return cmp.Compare(a.Code, b.Code) })
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = g.qualify(e.Pkg, g.names.MemberName(e, m))
	}
	v, m := g.temp(tempValue), g.temp(tempMember)
	if ir.OpenEnum(g.p, g.e, e) {
		fmt.Fprintf(b, openBitsFormat, v, g.typeName(e), m, strings.Join(names, listSep), mask, g.temp(tempInt), ir.GoCode)
	} else {
		fmt.Fprintf(b, bitsFormat, v, g.typeName(e), m, strings.Join(names, listSep), mask)
	}
	return g.rt() + makeList + v + rparen
}

// codeInt64 is the code of expr, a value of a @codes enum, as an int64: an opened enum's Code also reports whether it is known, which a value read by code always is (DECISIONS 339).
func (g *gen) codeInt64(e *ir.Enum, expr string) string {
	if !ir.OpenEnum(g.p, g.e, e) {
		return goInt64 + lparen + expr + dot + ir.GoCode + callSuffix + rparen
	}
	return fmt.Sprintf(openCodeFormat, g.typeName(e), ir.GoCode, expr)
}

func (g *gen) uint64Of(x string) string { return goUint64 + lparen + x + rparen }

// keyMember is the storage of a keyed list's key on its row: a ref key's key member.
func (g *gen) keyMember(rec *ir.Record, kf *ir.Field) string {
	if rec.Pkg != g.p.Name { // another package's record: its key getter (CODEGEN.md §5.8)
		s := g.names.Slot(kf)
		if s.Ref != nil {
			return s.KeyGetter + callSuffix
		}
		return s.Getter + callSuffix
	}
	for _, s := range g.bodyOf(rec).slots {
		if s.src == kf && s.Ref != nil {
			return s.KeyStore
		}
	}
	return g.names.Slot(kf).Store
}
