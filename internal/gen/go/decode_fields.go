package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// viaObject reports a field read through the object rather than the wire struct: a path of
// several keys, pairs slots, or a key encoding/json cannot name in a struct tag.
func viaObject(f *ir.Field) bool {
	return !f.Inline && (f.Pairs != nil || len(f.WirePath) != 1 || !tagSafe(f.WirePath[0]))
}

// tagSafe reports a key encoding/json reads from a struct tag unchanged (its isValidTag).
func tagSafe(key string) bool {
	if key == "" || key == tagSkip {
		return false
	}
	for _, c := range key {
		if !strings.ContainsRune(tagPunctuation, c) && !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

// readPath reads a field down its key path in the object; a missing or null object is its absence (WIRE.md §5.5.3).
func (g *gen) readPath(owner *body, s *slot) {
	f, lc := s.src, g.lc
	l := leaf{t: s.T, unit: f.Unit, enc: f.Enc}
	var marker []byte
	typ := g.wireType(l)
	if f.Optional && f.NoneWire != nil {
		marker, typ = f.NoneWire, g.rawType()
	}
	var b strings.Builder
	p := g.temp(tempPointer)
	fmt.Fprintf(&b, "var %s *%s\n", p, typ)
	obj, last := lc.Obj, len(f.WirePath)-1
	for i, seg := range f.WirePath {
		r, ok := g.temp(tempRaw), g.temp(tempOK)
		loc := g.keyLoc(strings.Join(f.WirePath[:i+1], dot))
		fmt.Fprintf(&b, lookupKeyFormat, r, ok, obj, strconv.Quote(seg), ok)
		if i < last {
			o := g.temp(tempObject)
			fmt.Fprintf(&b, openObjectFormat, o, lc.Err, helperObject, lc.Name, g.locExpr(loc.dot()), r)
			g.caseLoop(&b, o, g.locExpr(loc.dot()), nextSegments(owner, f.WirePath[:i+1]))
			obj = o
			continue
		}
		fmt.Fprintf(&b, readIntoFormat, lc.Err, g.use(jsonPath, jsonPkg), r, p, g.wrapAt(loc))
	}
	b.WriteString(strings.Repeat(closeBrace, len(f.WirePath)))
	key := strings.Join(f.WirePath, dot)
	g.readPtr(&b, ptrRead{s: s, l: l, marker: marker, ptr: p, recv: lc.Out, key: strconv.Quote(key), loc: g.keyLoc(key)})
	g.body.WriteString(b.String())
}

// nextSegments are the keys the field paths under prefix read next, the object's keys at prefix (WIRE.md §5.5.3).
func nextSegments(b *body, prefix []string) []string {
	var keys []string
	for _, s := range b.slots {
		if p := s.src; s.fn == nil && p != nil && len(p.WirePath) > len(prefix) && slices.Equal(p.WirePath[:len(prefix)], prefix) {
			keys = append(keys, p.WirePath[len(prefix)])
		}
	}
	return keys
}

// readPairs reads slots up to the first empty one, the record's fields at k(i) and v(i) (WIRE.md §5.14).
func (g *gen) readPairs(s *slot) {
	f, lc := s.src, g.lc
	rec, ok := g.sub(s.T.Elem).Named.(*ir.Record)
	if !ok || len(rec.Fields) != pairFields || f.Pairs.Slots <= 0 {
		g.failf(ErrMalformed, "pairs field %s without a two-field record or slots", s.origin)
		return
	}
	var keys [pairFields][]string
	for k, tmpl := range f.Pairs.Keys {
		keys[k] = g.slotKeys(tmpl, f.Pairs.Slots)
	}
	var b strings.Builder
	list, empty, i := g.temp(tempValue), g.temp(tempEmpty), g.temp(tempIndex)
	slotKeys := [pairFields]string{g.temp(tempKey), g.temp(tempKey)}
	raws := [pairFields]string{g.temp(tempRaw), g.temp(tempRaw)}
	oks := [pairFields]string{g.temp(tempOK), g.temp(tempOK)}
	elem := g.temp(tempElem)
	fmt.Fprintf(&b, pairsOpenFormat, list, g.goName(rec), empty, i, slotKeys[0], strings.Join(keys[0], listSep), slotKeys[1], strings.Join(keys[1], listSep))
	for k := range pairFields {
		fmt.Fprintf(&b, pairsRawFormat, raws[k], oks[k], lc.Obj, slotKeys[k])
	}
	at := g.root().arg(slotKeys[0])
	fmt.Fprintf(&b, pairsSlotFormat, oks[0], oks[1], empty, g.errAt(at, incompleteSlotText, ""), g.errAt(at, afterEmptyText, ""), elem, g.goName(rec))
	body := g.bodyOf(rec)
	if len(body.slots) < pairFields {
		g.failf(ErrMalformed, "pairs field %s of a record with a Never field", s.origin)
		return
	}
	for k, es := range body.slots[:pairFields] {
		p, l, loc := g.temp(tempPointer), leaf{t: es.T, unit: es.src.Unit, enc: es.src.Enc}, g.root().arg(slotKeys[k])
		fmt.Fprintf(&b, unmarshalFormat, p, pointer+g.wireType(l), lc.Err, g.use(jsonPath, jsonPkg), raws[k], g.wrapAt(loc))
		g.readPtr(&b, ptrRead{s: es, l: l, ptr: p, recv: elem, key: slotKeys[k], loc: loc})
	}
	fmt.Fprintf(&b, pairsCloseFormat, list, elem)
	g.store(&b, s, lc.Out, g.rt()+makeList+list+rparen)
	g.body.WriteString(b.String())
}

// pairsKeys are the keys of every slot of a pairs field.
func (g *gen) pairsKeys(f *ir.Field) []string {
	var keys []string
	for _, tmpl := range f.Pairs.Keys {
		for i := range f.Pairs.Slots {
			keys = append(keys, g.slotKey(tmpl, i))
		}
	}
	return keys
}

// slotKeys are a template's keys of slot 0 to n-1, quoted.
func (g *gen) slotKeys(tmpl string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = strconv.Quote(g.slotKey(tmpl, i))
	}
	return keys
}

// slotKey is a pairs template with `{i}` replaced by slot i in decimal (WIRE.md §5.14).
func (g *gen) slotKey(tmpl string, i int) string {
	before, after, found := strings.Cut(tmpl, lbrace)
	mark, rest, closed := strings.Cut(after, rbrace)
	if !found || !closed || mark != slotLetter {
		g.failf(ErrMalformed, "pairs template %s at %s", tmpl, g.at)
	}
	return before + strconv.Itoa(i) + rest
}

// tableWire is a lookup's `$<fn>` object, a level per parameter keyed by wire keys, cells read through pointers so null is seen (WIRE.md §5.11).
func (g *gen) tableWire(f *finiteMethod) string {
	return strings.Repeat(objectType, len(f.fn.Params)) + pointer + g.wireType(leaf{t: f.res.T})
}

// readTable fills a lookup's dense table from its nested object, in domain order (§5.10).
func (g *gen) readTable(b *strings.Builder, f *finiteMethod, ptr string) {
	store, pair := f.store, f.pair
	if f.res.Resolved {
		store, pair = f.res.KeyStore, f.res.Optional
	}
	src, loc, cell := lparen+pointer+ptr+rparen, g.keyLoc(dollar+f.fn.Name), g.lc.Out+dot+store
	for _, p := range f.fn.Params {
		g.caseLoop(b, src, g.locExpr(loc.dot()), g.domainWires(p.Type))
		i, k, m, ok := g.temp(tempIndex), g.temp(tempKey), g.temp(tempMember), g.temp(tempOK)
		fmt.Fprintf(b, tableLevelFormat, i, k, strings.Join(g.domainKeys(p.Type), listSep), m, ok, src, g.missing(loc.dot(), k))
		src, loc, cell = m, loc.dot().arg(k), cell+lbracket+i+rbracket
	}
	res := leaf{t: f.res.T}
	if f.res.Optional {
		fmt.Fprintf(b, ifOpenFormat, src+notNil)
	} else {
		fmt.Fprintf(b, ifNilReturnFormat, src, g.nullAt(loc))
	}
	x := g.conv(b, res, pointer+src, loc)
	if pair {
		fmt.Fprintf(b, cellPairFormat, cell, pairValue, pairOK, x)
	} else {
		fmt.Fprintf(b, cellFormat, cell, x)
	}
	if f.res.Optional {
		b.WriteString(closeBrace)
	}
	b.WriteString(strings.Repeat(closeBrace, len(f.fn.Params)))
}

// domainKeys are a finite parameter's wire keys in domain order (CODEGEN.md §5.10, WIRE.md §5.8).
func (g *gen) domainKeys(t ir.TypeRef) []string {
	keys := g.domainWires(t)
	for i, k := range keys {
		keys[i] = strconv.Quote(k)
	}
	return keys
}

// domainWires are a finite parameter's wire keys in domain order, unquoted.
func (g *gen) domainWires(t ir.TypeRef) []string {
	if t.Kind == types.Bool {
		return []string{strconv.FormatBool(false), strconv.FormatBool(true)}
	}
	e, ok := t.Named.(*ir.Enum)
	if !ok {
		return nil
	}
	keys := make([]string, len(e.Members))
	for i, m := range e.Members {
		keys[i] = m.Wire
		if e.JSONCodes {
			keys[i] = strconv.FormatInt(m.Code, decimal)
		}
	}
	return keys
}

// objectKeys are the keys of a record's or case's own object: its fields' first keys, pairs
// slots, `$` keys, and the tags of the variants written inline in it.
func (g *gen) objectKeys(b *body) []string {
	var keys []string
	for _, s := range b.slots {
		switch f := s.src; {
		case s.fn != nil:
			keys = append(keys, dollar+s.fn.Name)
		case f == nil:
		case f.Pairs != nil:
			keys = append(keys, g.pairsKeys(f)...)
		case f.Inline:
			if v, ok := s.T.Named.(*ir.Variant); ok {
				keys = append(keys, v.Tag)
			}
		case len(f.WirePath) > 0:
			keys = append(keys, f.WirePath[0])
		}
	}
	for _, f := range b.finite {
		keys = append(keys, dollar+f.fn.Name)
	}
	return keys
}

// foldCheck refuses two keys of one object equal but for case, which encoding/json confuses; extra
// are the object's other keys (a row's `$id`, a case's tag); an inline variant adds one case at a
// time.
func (g *gen) foldCheck(b *body, extra ...string) {
	keys := append(g.objectKeys(b), extra...)
	sets := [][]string{keys}
	for _, s := range b.slots {
		v, ok := s.T.Named.(*ir.Variant)
		if s.src == nil || !s.src.Inline || !ok {
			continue
		}
		for _, c := range v.Cases {
			if len(c.Fields) > 0 {
				sets = append(sets, append(slices.Clone(keys), g.objectKeys(g.caseBody(v, c))...))
			}
		}
	}
	for _, set := range sets {
		g.foldSet(set)
	}
}

func (g *gen) foldSet(keys []string) {
	for i, a := range keys {
		for _, b := range keys[i+1:] {
			if a != b && strings.EqualFold(a, b) {
				g.fail(newDetail(ErrUnsupported, g.at, foldFormat, a, b, g.at))
				return
			}
		}
	}
}
