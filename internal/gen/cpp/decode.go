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
	disc     string      // a dependent type's discriminant, read before it (CODEGEN.md §5.6); a ref's key for a table
	table    string      // a field decided through a ref: its branch table, which disc looks up (DECISIONS 350)
	def      value.Value // types mode: the constant default an absent key takes (CODEGEN.md §5.13)
	kept     string      // the static const a default holding a record or variant is built into (DECISIONS 386)
	nested   bool        // below a field's value and its list's elements: no hook member's key check reaches it (CODEGEN.md §2.8)
}

// decoders are the detail::Decode and Decode<Alias> definitions, in declaration order (CODEGEN.md §2.7, §5.6, §7.6).
func (g *gen) decoders() {
	for _, c := range g.declared() {
		leave := g.enter(c.canonName())
		if c.dependent != nil {
			g.dependentDecoder(c.dependent, g.pl.Dependent(c.dependent).Decode)
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
	g.decodeBody(fields, fns)
}

// decodeBody reads each field at its wire path, then each stored fn's `$` key, into what g.dst names.
func (g *gen) decodeBody(fields []*ir.Field, fns []*ir.ExportFn) {
	for _, f := range fields {
		g.decodeField(f, fields)
	}
	for _, fn := range fns {
		m, err := g.member(fn.Name)
		t, optional := resultType(fn.Result)
		l := leaf{t: t, optional: optional, dst: g.dst + m}
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
		g.decodePairs(f, g.dst+m)
		return
	case f.Inline && (f.Type.Kind != types.Variant || f.Optional):
		g.malformed(inlineFields, f.Name) // check's E3316 refuses it
		return
	case f.Inline:
		g.c.linef(1, decodeCallFormat, g.decodeFunc(f.Type), sourceVar, g.dst+m)
		return
	case len(f.WirePath) == 0:
		g.fail(fmt.Errorf("%w: field %s without a wire path", ErrMalformed, f.Name))
		return
	}
	l := leaf{t: f.Type, optional: f.Optional, none: f.NoneWire, unit: f.Unit, enc: f.Enc, dst: g.dst + m, def: g.fieldDefault(f)}
	l.kept = g.builtDefault(l, m)
	obj, depth := g.openPath(f, fields)
	if g.boxed[f] {
		l.box = g.storage(f.Type)
	}
	if app := ir.HeldApp(f.Type); app != nil { // its elements share the field's discriminant (CODEGEN.md §5.6)
		l.disc, l.table = g.leafDisc(fields, f, *app)
	}
	g.decodeKey(depth, obj, quote(f.WirePath[len(f.WirePath)-1]), l)
	g.closePath(f, depth, l)
	g.defineLookup(1, f, g.dst, quote(wireName(f)))
}

// openPath opens a path's intermediate objects, each on the decoder's path, and returns the innermost; a data loader also checks its keys (WIRE.md §5.5.3).
func (g *gen) openPath(f *ir.Field, fields []*ir.Field) (obj string, depth int) {
	obj, depth = sourceVar, 1
	for i, seg := range f.WirePath[:len(f.WirePath)-1] {
		v := fmt.Sprintf(pathVarFormat, i)
		g.c.linef(depth, stepOpenFormat, v, obj, quote(seg))
		g.c.linef(depth+1, pushFormat, stringLit(seg))
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
			g.assignDefault(depth+1, l)
		case !f.Optional:
			g.c.linef(depth, elseOpen)
			g.c.linef(depth+1, failMissingFormat, viewLit(strings.Join(f.WirePath[i:], qnameSep)))
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
	g.decodeValue(depth+1, fmt.Sprintf(derefFormat, x), key, leaf{t: l.t, unit: l.unit, enc: l.enc, dst: dst, disc: l.disc, table: l.table, nested: l.nested})
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
	case l.t.Kind == types.String, l.t.Kind == types.LitUnion && g.stringWire(l.t) && g.unionEnum(l.t) == nil && ir.UnionRef(l.t) == nil:
		return shortString
	case l.t.Kind == types.Duration && !g.types(): // types mode reads a source-wire Duration (WIRE.md §5.13)
		return shortDuration
	case g.openPlain(l.t) && l.enc == types.EncPlain:
		return shortOpenEnum
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
		if g.openPlain(l.t) {
			return ""
		}
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

// stringWire reports a literal union whose wire is always a JSON string: over a String, an enum without @json(codes), or a string-keyed ref (DECISIONS 387).
func (g *gen) stringWire(t ir.TypeRef) bool {
	if t.Elem == nil {
		return false
	}
	e, isEnum := t.Elem.Named.(*ir.Enum)
	return t.Elem.Kind == types.String || isEnum && !e.JSONCodes || ir.UnionRef(t) != nil
}

// decodeValue reads the JSON value src into l.dst (WIRE.md §5).
func (g *gen) decodeValue(depth int, src, key string, l leaf) {
	switch l.t.Kind {
	case types.Bool, types.Int, types.Float:
		g.decodeNumber(depth, src, key, l)
	case types.String:
		g.c.linef(depth, decCallFormat, asString, src, key, "", l.dst)
	case types.LitUnion:
		g.decodeUnion(depth, src, key, l)
	case types.Duration:
		g.decodeDuration(depth, src, key, l)
	case types.Enum:
		as := asEnum
		if g.openPlain(l.t) {
			as = asOpenEnum
		}
		g.c.linef(depth, decCallFormat, as, src, key, g.shortcutExtra(l), l.dst)
	case types.Ref:
		if id, ok := g.idEnum(l.t); ok {
			g.decodeEntryID(depth, src, key, id, l.dst)
			return
		}
		g.decodeValue(depth, src, key, leaf{t: *l.t.Key, dst: l.dst})
		if l.nested {
			g.foreignRefCheck(depth, l.t, l.t, l.dst, key)
		}
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
	case types.Optional:
		g.decodeOptional(depth, src, key, l)
	default:
		g.refuseKind(l.t.Kind, g.at)
	}
}

// decodeUnion reads a literal union's wire text, then checks it is a literal or a member of its enum, or, read by a data loader from another package's class, the key of an entry (CODEGEN.md §4.1, §5.13; DECISIONS 387).
func (g *gen) decodeUnion(depth int, src, key string, l leaf) {
	if !g.stringWire(l.t) {
		g.refuseUnion(l.t)
	}
	g.c.linef(depth, decCallFormat, asString, src, key, "", l.dst)
	g.unionMembership(depth, key, l.dst, l.t)
	g.foreignUnionCheck(depth, l.t, l.dst, key)
}

// decodeOptional reads an optional list element or map value: `null` is none, anything else its element (CODEGEN.md §4.2, WIRE.md §5.4; DECISIONS 385).
func (g *gen) decodeOptional(depth int, src, key string, l leaf) {
	if l.t.Elem == nil {
		g.malformed(noElem, g.at)
		return
	}
	dst := fmt.Sprintf(optVarFormat, depth)
	g.c.linef(depth, notNullOpenFormat, src)
	g.c.linef(depth+1, emplaceFormat, dst, l.dst)
	g.decodeValue(depth+1, src, key, leaf{t: *l.t.Elem, unit: l.unit, enc: l.enc, dst: dst, disc: l.disc, table: l.table, nested: true})
	g.c.linef(depth, closeBrace)
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
