package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// leaf is one JSON value to decode into a C++ lvalue: a field, a `$` key, an element.
type leaf struct {
	t        ir.TypeRef
	optional bool
	none     []byte // the @json(none:) marker
	unit     types.Unit
	enc      types.Enc
	dst      string // the C++ lvalue
	box      string // the class an optional std::unique_ptr holds, or ""
	cell     bool   // a lookup cell: absent is missing, null is none or an error (WIRE.md §5.11)
}

// decoders are the detail::Decode definitions, in declaration order (CODEGEN.md §2.7, §7.6).
func (g *gen) decoders() {
	for _, c := range g.declared() {
		leave := g.enter(c.canonName())
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
	g.checkKeys(1, sourceVar, g.objectKeys(c), true)
	if len(fields) == 0 && !hasStored(fns) {
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
		g.unsupported(inlineFields, f.Name)
		return
	case f.Inline:
		g.c.linef(1, decodeCallFormat, g.decodeFunc(f.Type), sourceVar, outPrefix+m)
		return
	case len(f.WirePath) == 0:
		g.fail(fmt.Errorf("%w: field %s without a wire path", ErrMalformed, f.Name))
		return
	}
	obj, depth := sourceVar, 1
	last := len(f.WirePath) - 1
	for i, seg := range f.WirePath[:last] {
		v := fmt.Sprintf(pathVarFormat, i)
		g.c.linef(depth, stepOpenFormat, v, obj, quote(seg))
		g.c.linef(depth+1, pushFormat, quote(seg))
		obj, depth = fmt.Sprintf(derefFormat, v), depth+1
		g.checkKeys(depth, obj, nextSegments(fields, f.WirePath[:i+1]), false)
	}
	l := leaf{t: f.Type, optional: f.Optional, none: f.NoneWire, unit: f.Unit, enc: f.Enc, dst: outPrefix + m}
	if g.boxed[f] {
		l.box = g.storage(f.Type)
	}
	g.decodeKey(depth, obj, quote(f.WirePath[last]), l)
	g.closePath(f, depth)
}

// closePath closes a path's intermediate objects: an absent one leaves a required field missing (WIRE.md §5.5.3).
func (g *gen) closePath(f *ir.Field, depth int) {
	for i := len(f.WirePath) - depthTwo; i >= 0; i-- {
		g.c.linef(depth, popLine)
		depth--
		if !f.Optional {
			g.c.linef(depth, elseOpen)
			g.c.linef(depth+1, failMissingFormat, quote(strings.Join(f.WirePath[i:], qnameSep)))
		}
		g.c.linef(depth, closeBrace)
	}
}

// decodeKey reads obj[key] into l (key a C++ `const char*` expression): a Decoder shortcut
// when one fits, else Required or Optional, then the value.
func (g *gen) decodeKey(depth int, obj, key string, l leaf) {
	if short := g.shortcut(l); short != "" && !l.optional && !l.cell {
		g.c.linef(depth, decCallFormat, short, obj, key, g.shortcutExtra(l), l.dst)
		return
	}
	x := fmt.Sprintf(jsonVarFormat, depth)
	switch {
	case l.cell:
		g.c.linef(depth, cellOpenFormat, x, obj, key, l.optional)
	case !l.optional:
		g.c.linef(depth, requiredOpenFormat, x, obj, key)
	case l.none != nil:
		g.c.linef(depth, markerOpenFormat, x, obj, key, x, g.markerTest(x, l.none))
	default:
		g.c.linef(depth, optionalOpenFormat, x, obj, key)
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
	g.decodeValue(depth+1, fmt.Sprintf(derefFormat, x), key, leaf{t: l.t, unit: l.unit, enc: l.enc, dst: dst})
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
	case l.t.Kind == types.String, l.t.Kind == types.LitUnion && g.stringWire(l.t):
		return shortString
	case l.t.Kind == types.Duration:
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
		return addressOf + g.typeName(e) + fromCodeSuffix
	}
	return addressOf + g.typeName(e) + fromWireSuffix
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
			g.unsupported(unionNonString, g.at)
		}
		g.c.linef(depth, decCallFormat, asString, src, key, "", l.dst)
	case types.Duration:
		g.c.linef(depth, decCallFormat, asDuration, src, key, g.shortcutExtra(l), l.dst)
	case types.Enum:
		g.c.linef(depth, decCallFormat, asEnum, src, key, g.shortcutExtra(l), l.dst)
	case types.Ref:
		g.decodeValue(depth, src, key, leaf{t: *l.t.Key, dst: l.dst})
	case types.Record, types.Variant:
		g.decodeObject(depth, src, key, l)
	case types.List:
		if l.enc == types.EncBits {
			g.decodeBits(depth, src, key, l)
			return
		}
		g.decodeList(depth, src, key, l)
	case types.Map:
		g.unsupported(mapFields, g.at)
	default:
		g.unsupported(kindText(l.t.Kind), g.at)
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
