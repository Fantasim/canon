package gogen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// decoders writes, in declaration order, a decoder per class a value holds (CODEGEN.md §6.1).
func (g *gen) decoders() {
	g.helpers()
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			if g.decoded[x] {
				g.decodeBody(g.bodyOf(x))
			}
		case *ir.Variant:
			if g.decoded[x] {
				g.decodeVariant(x)
				g.eachCase(x, func(c *ir.Case) bool { return g.decoded[c] }, g.decodeBody)
			}
		}
	}
}

// eachCase calls do on the body of each case with fields that keep accepts.
func (g *gen) eachCase(v *ir.Variant, keep func(*ir.Case) bool, do func(*body)) {
	for _, c := range v.Cases {
		if len(c.Fields) > 0 && keep(c) {
			do(g.caseBody(v, c))
		}
	}
}

// classGoName is the Go type of a record, variant or case.
func (g *gen) classGoName(key any) string {
	if c, ok := key.(*ir.Case); ok {
		if v := g.variantOf[c]; v != nil {
			return g.names.CaseName(v, c)
		}
	}
	if t, ok := key.(ir.Type); ok {
		return g.goName(t)
	}
	g.failf(ErrMalformed, "a class %T", key)
	return ""
}

func (g *gen) decodeFunc(key any) string  { return decodePrefix + g.classGoName(key) }
func (g *gen) resolveFunc(key any) string { return resolvePrefix + g.classGoName(key) }

// wireEntry is one key of a wire struct: a field of one key, or a stored fn's `$` key.
type wireEntry struct {
	name, key, typ string
	s              *slot
	fin            *finiteMethod
	l              leaf
	marker         []byte
}

// wireEntries are the one-key fields, then the `$` keys, each named like its getter (WIRE.md §5.5, §5.11).
func (g *gen) wireEntries(b *body) []wireEntry {
	var out []wireEntry
	for _, s := range b.slots {
		e := wireEntry{name: s.Getter, s: s, l: leaf{t: s.T}}
		if s.Ref != nil {
			e.name = s.KeyGetter
		}
		switch f := s.src; {
		case s.fn != nil:
			e.key = dollar + s.fn.Name
		case f == nil || f.Inline || viaObject(f):
			continue
		default:
			e.key, e.l.unit, e.l.enc = f.WirePath[0], f.Unit, f.Enc
			if f.Optional && f.NoneWire != nil {
				e.marker = f.NoneWire
			}
		}
		e.typ = g.entryType(e)
		out = append(out, e)
	}
	for _, f := range b.finite {
		out = append(out, wireEntry{name: f.name, key: dollar + f.fn.Name, fin: f, typ: g.tableWire(f)})
	}
	return out
}

func (g *gen) entryType(e wireEntry) string {
	if e.marker != nil {
		return g.rawType()
	}
	return g.wireType(e.l)
}

// decodeBody writes a record's or case's wire struct and decode function (CODEGEN.md §6.1).
func (g *gen) decodeBody(b *body) {
	defer g.enter(b.owner)()
	g.temps = 0
	entries := g.wireEntries(b)
	g.foldCheck(b, g.objectExtras(b)...)
	wire := wirePrefix + b.goName
	g.printf(structOpen, wire)
	for _, e := range entries {
		g.printf(wireFieldFormat, e.name, e.typ, e.key)
	}
	g.printf(closeBlock)
	lc := g.lc
	g.printf(funcOpenFormat, decodePrefix+b.goName, lc.Name, lc.Path, lc.Raw, g.rawType(), lc.Out, b.goName)
	g.openObject(g.expectedKeys(b))
	g.unmarshal(lc.W, wire, lc.Raw)
	g.requiredKeys(entries)
	var pairs []pair
	var stmts strings.Builder
	for _, e := range entries {
		pairs = g.readEntry(&stmts, pairs, e)
	}
	g.printf("*%s = %s\n%s", lc.Out, compositeLit(b.goName, pairs), stmts.String())
	for _, s := range b.slots {
		g.readOther(b, s)
	}
	g.printf(returnNil)
}

// objectExtras are an object's keys beside its fields: a row's `$id`, a case's tag (WIRE.md §5.6, §5.7).
func (g *gen) objectExtras(b *body) []string {
	if c, ok := b.key.(*ir.Case); ok && g.variantOf[c] != nil {
		return []string{g.variantOf[c].Tag}
	}
	if b.idType != "" {
		return []string{dollar + ir.GoIDStore, dollar + ir.GoRetiredStore}
	}
	return nil
}

// unmarshal declares v of type typ and reads raw into it; a failure names the path.
func (g *gen) unmarshal(v, typ, raw string) {
	g.printf(unmarshalFormat, v, typ, g.lc.Err, g.use(jsonPath, jsonPkg), raw, g.wrapAt(g.root()))
}

// requiredKeys fails on the first required key absent or null (CODEGEN.md §6.1, rt.Missing).
func (g *gen) requiredKeys(entries []wireEntry) {
	var cases strings.Builder
	for _, e := range entries {
		if e.fin != nil || !e.s.Optional {
			fmt.Fprintf(&cases, missingCaseFormat, g.lc.W, e.name, g.missing(g.root(), strconv.Quote(e.key)))
		}
	}
	if cases.Len() > 0 {
		g.printf("switch {\n%s}\n", cases.String())
	}
}

// missing is rt.Missing of the key expression key under the path prefix.
func (g *gen) missing(prefix location, key string) string {
	return fmt.Sprintf(missingFormat, g.rt(), g.lc.Name, g.locExpr(prefix), key)
}

// readEntry reads one wire-struct key: a pure required value joins the composite literal.
func (g *gen) readEntry(stmts *strings.Builder, pairs []pair, e wireEntry) []pair {
	ptr := g.lc.W + dot + e.name
	if e.fin != nil {
		g.readTable(stmts, e.fin, ptr)
		return pairs
	}
	if e.s.Optional {
		g.readPtr(stmts, ptrRead{s: e.s, l: e.l, marker: e.marker, ptr: ptr, recv: g.lc.Out, key: strconv.Quote(e.key), loc: g.keyLoc(e.key)})
		return pairs
	}
	var pre strings.Builder
	x := g.conv(&pre, e.l, pointer+ptr, g.keyLoc(e.key))
	if pre.Len() == 0 {
		return append(pairs, pair{target(e.s), x})
	}
	stmts.WriteString(pre.String())
	g.store(stmts, e.s, g.lc.Out, x)
	return pairs
}

// target is the member a decoded value goes to: a ref's key, else the value's storage.
func target(s *slot) string {
	if s.Ref != nil {
		return s.KeyStore
	}
	return s.Store
}

// store writes x to the slot's members on recv, marking an optional one present.
func (g *gen) store(b *strings.Builder, s *slot, recv, x string) {
	if s.needsOK() {
		fmt.Fprintf(b, assignOKFormat, recv, target(s), s.OKStore, x)
		return
	}
	fmt.Fprintf(b, assignFormat, recv, target(s), x)
}

// ptrRead is a wire pointer to decode into a slot: *W, or a raw value with a none marker;
// key is the Go expression of its key, loc its location.
type ptrRead struct {
	s              *slot
	l              leaf
	marker         []byte
	ptr, recv, key string
	loc            location
}

// readPtr reads an optional pointer, nil or the marker being none, or a required one (WIRE.md §5.4).
func (g *gen) readPtr(b *strings.Builder, r ptrRead) {
	loc := r.loc
	if !r.s.Optional {
		fmt.Fprintf(b, ifNilReturnFormat, r.ptr, g.missing(g.root(), r.key))
		g.store(b, r.s, r.recv, g.conv(b, r.l, pointer+r.ptr, loc))
		return
	}
	cond := r.ptr + notNil
	if r.marker != nil {
		cond += fmt.Sprintf(notMarkerFormat, r.ptr, strconv.Quote(string(r.marker)))
	}
	fmt.Fprintf(b, ifOpenFormat, cond)
	src := pointer + r.ptr
	if r.marker != nil {
		src = g.temp(tempValue)
		fmt.Fprintf(b, markedFormat, src, g.wireType(r.l), g.lc.Err, g.use(jsonPath, jsonPkg), r.ptr, g.wrapAt(loc))
	}
	g.store(b, r.s, r.recv, g.conv(b, r.l, src, loc))
	b.WriteString(closeBrace)
}

// decodeVariant reads the tag, then the case's fields from the same object (WIRE.md §5.6).
func (g *gen) decodeVariant(v *ir.Variant) {
	defer g.enter(v.QName())()
	if v.Tag == "" {
		g.failf(ErrMalformed, "variant %s without a tag", v.QName())
	}
	lc, name := g.lc, g.goName(v)
	g.printf(funcOpenFormat, g.decodeFunc(v), lc.Name, lc.Path, lc.Raw, g.rawType(), lc.Out, name)
	g.openObject([]string{v.Tag})
	raw, ok := g.temp(tempRaw), g.temp(tempOK)
	g.printf(tagReadFormat, raw, lc.Obj, strconv.Quote(v.Tag), g.missing(g.root(), strconv.Quote(v.Tag)),
		lc.Tag, lc.Err, g.use(jsonPath, jsonPkg), g.wrapAt(g.keyLoc(v.Tag)), ok)
	g.printf(switchTagFormat, lc.Tag)
	for i, c := range v.Cases {
		g.printf(caseFormat, strconv.Quote(c.Wire))
		kind := g.kindLit(v, i)
		if len(c.Fields) == 0 {
			g.printf(kindOnlyFormat, lc.Out, name, ir.GoKindStore, kind)
			continue
		}
		g.printf(decodeIntoFormat, lc.C, g.names.CaseName(v, c), lc.Err, g.decodeFunc(c), lc.Name, lc.Path, lc.Raw)
		g.printf(kindCaseFormat, lc.Out, name, ir.GoKindStore, kind, ir.GoCaseStore, lc.C)
	}
	g.printf(unknownCaseFormat, g.errAt(g.keyLoc(v.Tag), unknownCaseText, lc.Tag))
	g.printf(returnNil)
}

// readOther reads an inline variant, a key path or pairs slots, outside the wire struct (WIRE.md §5.5.3, §5.6, §5.14).
func (g *gen) readOther(b *body, s *slot) {
	f := s.src
	switch {
	case f == nil || s.fn != nil:
	case f.Inline:
		g.readInline(s)
	case f.Pairs != nil:
		g.readPairs(s)
	case viaObject(f):
		g.readPath(b, s)
	}
}

// readInline decodes a variant written in the parent object: the tag and the case's fields.
func (g *gen) readInline(s *slot) {
	if s.Optional || s.T.Kind != types.Variant {
		g.fail(newDetail(ErrUnsupported, s.origin, inlineFormat, s.origin))
		return
	}
	var b strings.Builder
	v := g.temp(tempValue)
	fmt.Fprintf(&b, decodeIntoFormat, v, g.goName(s.T.Named), g.lc.Err, g.decodeFunc(s.T.Named), g.lc.Name, g.lc.Path, g.lc.Raw)
	g.store(&b, s, g.lc.Out, v)
	g.body.WriteString(b.String())
}
