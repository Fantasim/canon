package gogen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// decoders writes the loaders' helpers, then a decoder per class a value holds (CODEGEN.md §6.1).
func (g *gen) decoders() {
	if len(g.decoded) == 0 {
		return
	}
	outer := g.body
	g.body = bytes.Buffer{}
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
	decoders := g.body
	g.body = outer
	g.helpers()
	g.body.Write(decoders.Bytes())
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

// decodeBody writes a record's or case's decoder: its object and key case, each field in
// declaration order, then each stored fn's `$` key in method order, as the C++ loader reads them.
func (g *gen) decodeBody(b *body) {
	defer g.enter(b.owner)()
	g.temps = 0
	g.inlineFolds(b)
	lc := g.lc
	g.printf(funcOpenFormat, decodePrefix+b.goName, lc.Name, lc.Path, lc.Raw, g.rawType(), lc.Out, b.goName)
	g.openObject(g.expectedKeys(b))
	for _, s := range b.slots {
		if s.fn == nil {
			g.readField(b, s)
		}
	}
	for _, fn := range b.methods {
		g.readMethod(b, fn)
	}
	g.printf(returnNil)
}

// readField reads one field where its wire form puts it (WIRE.md §5.5, §5.6, §5.14).
func (g *gen) readField(owner *body, s *slot) {
	f := s.src
	switch {
	case f.Inline:
		g.readInline(s)
	case f.Pairs != nil:
		g.readPairs(s)
	case len(f.WirePath) > 1:
		g.readPath(owner, s)
	case len(f.WirePath) == 1:
		g.readKey(s, g.lc.Obj, g.root(), f.WirePath[0])
	default:
		g.failf(ErrMalformed, "field %s without a wire path", s.origin)
	}
}

// readMethod reads a stored fn's `$` key: a precomputed result, or a lookup's table (WIRE.md §5.11).
func (g *gen) readMethod(b *body, fn *ir.ExportFn) {
	for _, s := range b.slots {
		if s.fn == fn {
			g.readKey(s, g.lc.Obj, g.root(), dollar+fn.Name)
		}
	}
	for _, f := range b.finite {
		if f.fn == fn {
			g.readTable(f)
		}
	}
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

// slotLeaf is what a slot reads: its type after the optional, its field's unit, encoding and none marker.
func slotLeaf(s *slot) (leaf, []byte) {
	l := leaf{t: s.T}
	if f := s.src; s.fn == nil && f != nil {
		l.unit, l.enc = f.Unit, f.Enc
		if f.Optional {
			return l, f.NoneWire
		}
	}
	return l, nil
}

// readKey reads key of obj, the object at at, into the slot; optional: absent, null or the marker is none (WIRE.md §5.4).
func (g *gen) readKey(s *slot, obj string, at location, key string) {
	lc := g.lc
	l, marker := slotLeaf(s)
	r, loc, quoted := g.temp(tempRaw), at.key(key), strconv.Quote(key)
	if !s.Optional {
		g.printf(needFormat, r, lc.Err, g.helper(helperNeed), lc.Name, g.locExpr(at), obj, quoted)
		g.storeRead(s, l, r, loc)
		return
	}
	cond := ""
	if marker != nil {
		cond = g.markerTest(r, marker)
	}
	g.printf(mayFormat, r, g.helper(helperMay), obj, quoted, cond)
	g.storeRead(s, l, r, loc)
	g.printf(closeBrace)
}

// storeRead reads raw into the slot's members on out.
func (g *gen) storeRead(s *slot, l leaf, raw string, loc location) {
	var b strings.Builder
	g.store(&b, s, g.lc.Out, g.readValue(&b, l, raw, loc))
	g.body.WriteString(b.String())
}

// markerTest is the condition that raw is not the field's none marker, JSON-equal as the C++
// loader compares them; a non-empty object or array marker is refused, as C++ refuses it.
func (g *gen) markerTest(raw string, marker []byte) string {
	var v any
	if json.Unmarshal(marker, &v) == nil && nonEmpty(v) {
		g.fail(newDetail(ErrUnsupported, g.at, noneMarkerFormat, g.at, marker))
	}
	return notMarkerPrefix + g.helper(helperSame) + lparen + raw + listSep + strconv.Quote(string(bytes.TrimSpace(marker))) + rparen
}

// nonEmpty reports a decoded JSON object or array that holds something.
func nonEmpty(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		return len(x) > 0
	case []any:
		return len(x) > 0
	}
	return false
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

// decodeVariant reads the tag, then the case's fields from the same object (WIRE.md §5.6).
func (g *gen) decodeVariant(v *ir.Variant) {
	defer g.enter(v.QName())()
	if v.Tag == "" {
		g.failf(ErrMalformed, "variant %s without a tag", v.QName())
	}
	g.temps = 0
	lc, name := g.lc, g.goName(v)
	g.printf(funcOpenFormat, g.decodeFunc(v), lc.Name, lc.Path, lc.Raw, g.rawType(), lc.Out, name)
	g.openObject([]string{v.Tag})
	loc := g.keyLoc(v.Tag)
	r := g.temp(tempRaw)
	g.printf(needFormat, r, lc.Err, g.helper(helperNeed), lc.Name, lc.Path, lc.Obj, strconv.Quote(v.Tag))
	g.readPlainInto(lc.Tag, goString, r, loc)
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
	g.printf(unknownCaseFormat, g.errAt(loc, unknownCaseText, lc.Tag))
	g.printf(returnNil)
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
