package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// leaf is one JSON value to decode into a C++ lvalue: a field, a `$` key, an element.
type leaf struct {
	t        ir.TypeRef
	optional bool
	none     []byte // the @json(none:) marker
	unit     types.Unit
	enc      types.Enc
	dst      string      // the C++ lvalue
	box      string      // the class an optional std::unique_ptr holds, or ""
	cell     bool        // a lookup cell: absent is missing, null is none or an error (WIRE.md §5.11)
	disc     string      // a dependent type's discriminant, read before it (CODEGEN.md §5.6)
	def      value.Value // types mode: the constant default an absent key takes (CODEGEN.md §5.13)
}

// decoders are the detail::Decode and Decode<Alias> definitions, in declaration order (CODEGEN.md §2.7, §5.6, §7.6).
func (g *gen) decoders() {
	for _, c := range g.declared() {
		leave := g.enter(c.canonName())
		if c.dependent != nil {
			g.dependentDecoder(c.dependent)
			leave()
			continue
		}
		name := g.className(c)
		g.c.printf(decodeOpenFormat, name)
		if c.variant != nil && c.cs == nil {
			g.decodeVariant(c.variant)
		} else {
			g.decodeRecord(c)
		}
		g.c.linef(1, returnOk)
		g.c.line(closeBrace)
		g.c.blank()
		leave()
	}
}

// decodeRecord reads each field in declaration order, then the `$` keys of the stored fns (§7.6, WIRE.md §5.11).
func (g *gen) decodeRecord(c class) {
	fields, fns := c.shape()
	g.inlineFolds(c)
	g.checkKeys(1, sourceVar, g.wireKeys(g.objectKeys(c)), true)
	if !anyFieldDecodes(fields) && !hasStored(fns) {
		g.c.linef(1, unusedFormat, outVar)
	}
	for _, f := range fields {
		g.decodeField(f, fields)
	}
	for _, fn := range fns {
		m, err := g.member(fn.Name)
		t, optional := resultType(fn.Result)
		l := leaf{t: t, optional: optional, dst: outPrefix + m}
		switch fn.Kind {
		case ir.FnPrecomputed:
			g.fail(err)
			g.decodeKey(1, sourceVar, quote(dollar+fn.Name), l)
		case ir.FnLookup:
			g.fail(err)
			g.decodeTable(fn, l)
		default:
		}
	}
}

func hasStored(fns []*ir.ExportFn) bool {
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			return true
		}
	}
	return false
}

// anyFieldDecodes reports a field decodeField writes to out (CODEGEN.md §5.12, §7.6).
func anyFieldDecodes(fields []*ir.Field) bool {
	for _, f := range fields {
		if f.Input == nil && (f.Type.Kind != types.Never || !f.Optional) {
			return true
		}
	}
	return false
}

// decodeField reads one field at its wire path, fields its record's; an inline variant reads the parent object (WIRE.md §5.5, §5.6).
func (g *gen) decodeField(f *ir.Field, fields []*ir.Field) {
	leave := g.enter(g.at + qnameSep + f.Name)
	defer leave()
	if f.Input != nil || f.Type.Kind == types.Never && f.Optional {
		return
	}
	m, err := g.member(f.Name)
	g.fail(err)
	switch {
	case f.Pairs != nil:
		g.decodePairs(f, outPrefix+m)
		return
	case f.Inline && (f.Type.Kind != types.Variant || f.Optional):
		g.malformed(inlineFields, f.Name) // check's E3316 refuses it
		return
	case f.Inline:
		g.c.linef(1, decodeCallFormat, g.decodeFunc(f.Type), sourceVar, outPrefix+m)
		return
	case len(f.WirePath) == 0:
		g.fail(fmt.Errorf("%w: field %s without a wire path", ErrMalformed, f.Name))
		return
	}
	obj, depth := g.openPath(f, fields)
	l := leaf{t: f.Type, optional: f.Optional, none: f.NoneWire, unit: f.Unit, enc: f.Enc, dst: outPrefix + m, def: g.fieldDefault(f)}
	if g.boxed[f] {
		l.box = g.storage(f.Type)
	}
	if app := ir.HeldApp(f.Type); app != nil { // its elements share the field's discriminant (CODEGEN.md §5.6)
		l.disc = g.discExpr(fields, *app)
	}
	g.decodeKey(depth, obj, quote(f.WirePath[len(f.WirePath)-1]), l)
	g.closePath(f, depth, l)
	g.defineLookup(1, f, outPrefix, quote(wireName(f)))
}

// openPath opens a path's intermediate objects, each on the decoder's path, and returns the innermost; a data loader also checks its keys (WIRE.md §5.5.3).
func (g *gen) openPath(f *ir.Field, fields []*ir.Field) (obj string, depth int) {
	obj, depth = sourceVar, 1
	for i, seg := range f.WirePath[:len(f.WirePath)-1] {
		v := fmt.Sprintf(pathVarFormat, i)
		g.c.linef(depth, stepOpenFormat, v, obj, quote(seg))
		g.c.linef(depth+1, pushFormat, quote(seg))
		obj, depth = fmt.Sprintf(derefFormat, v), depth+1
		if !g.types() {
			g.checkKeys(depth, obj, nextSegments(fields, f.WirePath[:i+1]), false)
		}
	}
	return obj, depth
}

// closePath closes a path's intermediate objects: an absent one leaves the field absent, so a required field missing, or, in types mode, its default (WIRE.md §5.5.3; CODEGEN.md §5.13).
func (g *gen) closePath(f *ir.Field, depth int, l leaf) {
	for i := len(f.WirePath) - depthTwo; i >= 0; i-- {
		g.c.linef(depth, popLine)
		depth--
		switch {
		case l.def != nil:
			g.c.linef(depth, elseOpen)
			g.c.linef(depth+1, assignFormat, l.dst, g.defaultLit(l.t, l.def))
		case !f.Optional:
			g.c.linef(depth, elseOpen)
			g.c.linef(depth+1, failMissingFormat, quote(strings.Join(f.WirePath[i:], qnameSep)))
		}
		g.c.linef(depth, closeBrace)
	}
}

// decodeKey reads obj[key] into l (key a C++ `const char*` expression): a Decoder shortcut
// when one fits, else Required or Optional, then the value.
func (g *gen) decodeKey(depth int, obj, key string, l leaf) {
	lead := g.absentDefault(depth, obj, key, l)
	if short := g.shortcut(l); short != "" && !l.optional && !l.cell && lead == "" {
		g.c.linef(depth, decCallFormat, short, obj, key, g.shortcutExtra(l), l.dst)
		return
	}
	x := fmt.Sprintf(jsonVarFormat, depth)
	switch {
	case l.cell:
		g.c.linef(depth, lead+cellOpenFormat, x, obj, key, l.optional)
	case !l.optional:
		g.c.linef(depth, lead+requiredOpenFormat, x, obj, key)
	case l.none != nil:
		g.c.linef(depth, lead+markerOpenFormat, x, obj, key, x, g.markerTest(x, l.none))
	default:
		g.c.linef(depth, lead+optionalOpenFormat, x, obj, key)
	}
	dst := l.dst
	switch {
	case l.box != "":
		dst = fmt.Sprintf(optVarFormat, depth)
		g.c.linef(depth+1, boxFormat, dst, l.dst, l.box)
	case l.optional:
		dst = fmt.Sprintf(optVarFormat, depth)
		g.c.linef(depth+1, emplaceFormat, dst, l.dst)
	}
	g.decodeValue(depth+1, fmt.Sprintf(derefFormat, x), key, leaf{t: l.t, unit: l.unit, enc: l.enc, dst: dst, disc: l.disc})
	g.c.linef(depth, closeBrace)
}

// shortcut is the Decoder field shortcut that reads l's type into its own storage (§7.5), or "".
func (g *gen) shortcut(l leaf) string {
	switch {
	case l.t.Kind == types.Bool && l.enc == types.EncPlain:
		return shortBool
	case l.t.Kind == types.Int && l.t.Bits == bits64 && l.t.Signed:
		return shortInt
	case l.t.Kind == types.Float && l.t.Bits != float32Bits:
		return shortFloat
	case l.t.Kind == types.String, l.t.Kind == types.LitUnion && g.stringWire(l.t) && g.unionEnum(l.t) == nil:
		return shortString
	case l.t.Kind == types.Duration && !g.types(): // types mode reads a source-wire Duration (WIRE.md §5.13)
		return shortDuration
	case l.t.Kind == types.Enum && l.enc == types.EncPlain:
		return shortEnum
	default:
		return ""
	}
}

// shortcutExtra is the argument a shortcut takes before the destination: a unit, a parser.
func (g *gen) shortcutExtra(l leaf) string {
	switch l.t.Kind {
	case types.Duration:
		return fmt.Sprintf(unitArgFormat, l.unit.Millis())
	case types.Enum:
		return g.parser(l.t) + listSep
	default:
		return ""
	}
}

// parser is the enum's FromCode with @json(codes), else its FromWire (WIRE.md §5.3).
func (g *gen) parser(t ir.TypeRef) string {
	e, ok := t.Named.(*ir.Enum)
	if !ok {
		g.fail(fmt.Errorf("%w: enum type without its enum at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	if e.JSONCodes {
		return addressOf + g.enumHelpers(e).FromCode
	}
	return addressOf + g.enumHelpers(e).FromWire
}

// stringWire reports a literal union whose wire is always a JSON string.
func (g *gen) stringWire(t ir.TypeRef) bool {
	if t.Elem == nil {
		return false
	}
	e, isEnum := t.Elem.Named.(*ir.Enum)
	return t.Elem.Kind == types.String || isEnum && !e.JSONCodes
}

// decodeValue reads the JSON value src into l.dst (WIRE.md §5).
func (g *gen) decodeValue(depth int, src, key string, l leaf) {
	switch l.t.Kind {
	case types.Bool, types.Int, types.Float:
		g.decodeNumber(depth, src, key, l)
	case types.String:
		g.c.linef(depth, decCallFormat, asString, src, key, "", l.dst)
	case types.LitUnion:
		if !g.stringWire(l.t) {
			g.refuseUnion(l.t)
		}
		g.c.linef(depth, decCallFormat, asString, src, key, "", l.dst)
		g.unionMembership(depth, key, l.dst, l.t)
	case types.Duration:
		g.decodeDuration(depth, src, key, l)
	case types.Enum:
		g.c.linef(depth, decCallFormat, asEnum, src, key, g.shortcutExtra(l), l.dst)
	case types.Ref:
		g.decodeValue(depth, src, key, leaf{t: *l.t.Key, dst: l.dst})
	case types.Record, types.Variant:
		g.decodeObject(depth, src, key, l)
	case types.TypeApp:
		g.decodeDependent(depth, src, key, l)
	case types.List:
		if l.enc == types.EncBits {
			g.decodeBits(depth, src, key, l)
			return
		}
		g.decodeList(depth, src, key, l)
	case types.Table:
		g.decodeNested(depth, src, key, l)
	case types.Map:
		g.decodeMap(depth, src, key, l)
	default:
		g.refuseKind(l.t.Kind, g.at, typeRefused)
	}
}

// decodeNumber reads Bool, integers and floats; a narrower type goes through an int64 or a double.
func (g *gen) decodeNumber(depth int, src, key string, l leaf) {
	switch {
	case l.t.Kind == types.Bool && l.enc == types.EncInt:
		g.c.linef(depth, intTempLine)
		g.c.linef(depth, intBoolFormat, src, key, l.dst)
	case l.t.Kind == types.Bool:
		g.c.linef(depth, decCallFormat, asBool, src, key, "", l.dst)
	case l.t.Kind == types.Int && l.t.Bits == bits64 && l.t.Signed:
		g.c.linef(depth, decCallFormat, asInt, src, key, "", l.dst)
	case l.t.Kind == types.Int:
		lo, hi := intRange(l.t)
		g.c.linef(depth, intTempLine)
		g.c.linef(depth, narrowIntFormat, src, key, lo, hi, l.dst, intType(l.t))
	case l.t.Bits == float32Bits:
		g.c.linef(depth, asFloat32Format, src, key, l.dst)
	default:
		g.c.linef(depth, decCallFormat, asFloat, src, key, "", l.dst)
	}
}

// decodeObject decodes a record or variant, with its key on the decoder's path.
func (g *gen) decodeObject(depth int, src, key string, l leaf) {
	g.c.linef(depth, pushFormat, key)
	g.c.linef(depth, decodeCallFormat, g.decodeFunc(l.t), src, l.dst)
	g.c.linef(depth, popLine)
}
