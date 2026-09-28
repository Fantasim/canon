package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// readPath reads a field down its key path; an absent object leaves the field missing (WIRE.md §5.5.3).
func (g *gen) readPath(owner *body, s *slot) {
	f, lc := s.src, g.lc
	obj, last := lc.Obj, len(f.WirePath)-1
	levels := []location{g.root()}
	for i, seg := range f.WirePath[:last] {
		r, ok, o := g.temp(tempRaw), g.temp(tempOK), g.temp(tempObject)
		next := levels[i].key(seg).dot()
		g.printf(lookupKeyFormat, r, ok, obj, strconv.Quote(seg), ok)
		g.printf(openObjectFormat, o, lc.Err, g.helper(helperObject), lc.Name, g.locExpr(next), r)
		g.keysCheck(o, next, nextSegments(owner, f.WirePath[:i+1]))
		obj, levels = o, append(levels, next)
	}
	g.readKey(owner, s, obj, levels[last], f.WirePath[last])
	for i := last - 1; i >= 0; i-- {
		if !s.Optional {
			g.printf(elseReturnFormat, g.missing(levels[i], strconv.Quote(strings.Join(f.WirePath[i:], dot))))
		}
		g.printf(closeBrace)
	}
}

// nextSegments are the keys the field paths under prefix read next, the object's keys at prefix (WIRE.md §5.5.3).
func nextSegments(b *body, prefix []string) []string {
	var keys []string
	for _, s := range b.slots {
		p := s.src
		if s.fn != nil || p == nil || s.isInput() {
			continue
		}
		if len(p.WirePath) > len(prefix) && slices.Equal(p.WirePath[:len(prefix)], prefix) {
			keys = append(keys, p.WirePath[len(prefix)])
		}
	}
	return keys
}

// readPairs reads the filled slots, the record's fields at k(i) and v(i) (WIRE.md §5.14).
func (g *gen) readPairs(s *slot) {
	f, lc := s.src, g.lc
	rec, ok := g.sub(s.T.Elem).Named.(*ir.Record)
	if !ok || len(rec.Fields) != pairFields || f.Pairs.Slots <= 0 || len(g.bodyOf(rec).slots) < pairFields {
		g.failf(ErrMalformed, "pairs field %s without a two-field record or slots", s.origin)
		return
	}
	var keys [pairFields]string
	for k, tmpl := range f.Pairs.Keys {
		keys[k] = strings.Join(g.slotKeys(tmpl, f.Pairs.Slots), listSep)
	}
	list, empty, i := g.temp(tempValue), g.temp(tempEmpty), g.temp(tempIndex)
	slotKeys := [pairFields]string{g.temp(tempKey), g.temp(tempKey)}
	raws := [pairFields]string{g.temp(tempRaw), g.temp(tempRaw)}
	elem := g.temp(tempElem)
	g.printf(pairsOpenFormat, list, g.goName(rec), empty, i, slotKeys[0], keys[0], slotKeys[1], keys[1])
	g.printf(slotFormat, raws[0], raws[1], lc.Err, g.helper(helperSlot), lc.Name, lc.Path, lc.Obj, slotKeys[0], slotKeys[1], empty, elem, g.goName(rec))
	for k, es := range g.bodyOf(rec).slots[:pairFields] {
		g.storeInto(es, elem, leaf{t: es.T, unit: es.src.Unit, enc: es.src.Enc}, raws[k], g.root().arg(slotKeys[k]))
		g.readDefine(es, elem, g.root().arg(slotKeys[k]))
	}
	g.printf(pairsCloseFormat, list, elem)
	var b strings.Builder
	g.store(&b, s, lc.Out, g.rt()+makeList+list+rparen)
	g.body.WriteString(b.String())
}

// storeInto reads raw into the slot's members on recv.
func (g *gen) storeInto(s *slot, recv string, l leaf, raw string, loc location) {
	var b strings.Builder
	g.store(&b, s, recv, g.readValue(&b, l, raw, loc))
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

// readTable fills a lookup's table from its `$<fn>` object, a level per parameter (WIRE.md §5.11).
func (g *gen) readTable(f *finiteMethod) {
	lc := g.lc
	store, pair := f.store, f.pair
	if f.res.Resolved {
		store, pair = f.res.KeyStore, f.res.Optional
	}
	key := dollar + f.fn.Name
	r, obj, prefix := g.temp(tempRaw), g.temp(tempObject), g.temp(tempPrefix)
	g.printf(needFormat, r, lc.Err, g.helper(helperNeed), lc.Name, lc.Path, lc.Obj, strconv.Quote(key))
	g.printf(prefixFormat, prefix, g.locExpr(g.root().key(key).dot()))
	g.printf(openObjectFormat, obj, lc.Err, g.helper(helperObject), lc.Name, prefix, r)
	cell, last := lc.Out+dot+store, len(f.fn.Params)-1
	for n, p := range f.fn.Params {
		g.keysCheck(obj, prefixLoc(prefix), g.domainWires(p.Type))
		i, k := g.temp(tempIndex), g.temp(tempKey)
		g.printf(domainLoopFormat, i, k, strings.Join(g.domainKeys(p.Type), listSep))
		cell += lbracket + i + rbracket
		if n == last {
			g.readCell(f, obj, prefixLoc(prefix).arg(k), cell, pair)
			break
		}
		r, next, o := g.temp(tempRaw), g.temp(tempPrefix), g.temp(tempObject)
		g.printf(needFormat, r, lc.Err, g.helper(helperNeed), lc.Name, prefix, obj, k)
		g.printf(prefixFormat, next, g.locExpr(prefixLoc(prefix).arg(k).dot()))
		g.printf(openObjectFormat, o, lc.Err, g.helper(helperObject), lc.Name, next, r)
		obj, prefix = o, next
	}
	g.body.WriteString(strings.Repeat(closeBrace, len(f.fn.Params)))
}

// prefixLoc is the location of what the local prefix, a path with its dot, holds.
func prefixLoc(prefix string) location { return location{format: verbString, args: []string{prefix}} }

// readCell reads one cell of the level obj; null is none for an optional result (WIRE.md §5.11).
func (g *gen) readCell(f *finiteMethod, obj string, loc location, cell string, pair bool) {
	lc := g.lc
	prefix, key := g.splitLoc(loc)
	r := g.temp(tempRaw)
	var b strings.Builder
	if f.res.Optional {
		g.printf(cellFormat, r, lc.Err, g.helper(helperCell), lc.Name, prefix, obj, key, r)
	} else {
		g.printf(needFormat, r, lc.Err, g.helper(helperNeed), lc.Name, prefix, obj, key)
	}
	x := g.readValue(&b, leaf{t: f.res.T}, r, loc)
	if pair {
		fmt.Fprintf(&b, cellPairFormat, cell, pairValue, pairOK, x)
	} else {
		fmt.Fprintf(&b, cellStoreFormat, cell, x)
	}
	if f.res.Optional {
		b.WriteString(closeBrace)
	}
	g.body.WriteString(b.String())
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
		case f == nil, s.isInput():
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

// inlineFolds refuses an inline variant a key of which equals another key of its parent but
// for ASCII letter case: its case decoder reads the parent's object, as gen/cpp refuses it.
func (g *gen) inlineFolds(b *body) {
	all := append(g.objectKeys(b), g.objectExtras(b)...)
	for _, s := range b.slots {
		v, ok := s.T.Named.(*ir.Variant)
		if s.src == nil || !s.src.Inline || !ok {
			continue
		}
		own := []string{v.Tag}
		g.eachCase(v, func(*ir.Case) bool { return true }, func(c *body) { own = append(own, g.objectKeys(c)...) })
		folds := func(k string) bool {
			return slices.ContainsFunc(all, func(p string) bool { return !slices.Contains(own, p) && asciiFold(k, p) })
		}
		if slices.ContainsFunc(own, folds) {
			g.fail(newDetail(ErrMalformed, s.origin, foldFormat, s.origin)) // E8019 InlineFoldedKey
			return
		}
	}
}
