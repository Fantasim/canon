package gogen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// resolvers writes resolve<T> per class holding a ref resolved at load, itself or deeper (CODEGEN.md §5.8, §5.11).
func (g *gen) resolvers() {
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			if g.names.NeedsWalk(x) {
				g.resolveBody(g.bodyOf(x))
			}
		case *ir.Variant:
			if g.names.NeedsWalk(x) {
				g.resolveVariant(x)
				g.eachCase(x, func(c *ir.Case) bool { return g.names.NeedsWalk(c) }, g.resolveBody)
			}
		}
	}
}

// ctxOf is where a class's refs find their entries, the snapshot or its one holder's container (CODEGEN.md §5.8).
func (g *gen) ctxOf(key any) (typ string, snapshot bool) {
	holders := g.names.Holders(key)
	if len(holders) == 0 || holders[0].Reload {
		return pointer + g.names.Snapshot().Type, true
	}
	return pointer + g.names.ContainerName(holders[0]), false
}

func (g *gen) resolveOpen(key any, goName string) (snapshot bool) {
	g.temps = 0
	ctx, snapshot := g.ctxOf(key)
	lc := g.lc
	g.printf(funcOpenFormat, g.resolveFunc(key), lc.Name, lc.Path, lc.Ctx, ctx, lc.Out, goName)
	return snapshot
}

// resolveVariant resolves the case a variant holds, when that case holds a ref to resolve.
func (g *gen) resolveVariant(v *ir.Variant) {
	defer g.enter(v.QName())()
	g.resolveOpen(v, g.goName(v))
	lc := g.lc
	g.printf(switchCaseFormat, lc.C, lc.Out, ir.GoCaseStore)
	for _, c := range v.Cases {
		if len(c.Fields) > 0 && g.names.NeedsWalk(c) {
			g.printf(resolveCaseFormat, g.names.CaseName(v, c), g.resolveFunc(c), lc.Name, lc.Path, lc.Ctx, lc.C)
		}
	}
	g.printf(closeBrace + returnNil)
}

// resolveBody resolves a record's or case's refs, then walks the classes its fields hold.
func (g *gen) resolveBody(b *body) {
	defer g.enter(b.owner)()
	snapshot := g.resolveOpen(b.key, b.goName)
	var out strings.Builder
	for _, s := range b.slots {
		if s.Resolved {
			g.resolveSlot(&out, s, g.slotTarget(s), snapshot)
			continue
		}
		switch {
		case s.Ref != nil:
		case s.src != nil && s.src.Pairs != nil:
			g.walkPairs(&out, s, snapshot)
		case g.typeWalks(s.T):
			g.walkSlot(&out, s)
		}
	}
	for _, f := range b.finite {
		if f.res.Resolved {
			g.resolveCells(&out, f, g.slotTarget(f.res), snapshot)
		}
		if g.typeWalks(f.res.T) {
			g.fail(newDetail(ErrUnsupported, f.origin, lookupRefFormat, f.origin))
		}
	}
	g.body.WriteString(out.String())
	g.printf(returnNil)
}

// messageKey is the key a slot's value has in its object, for messages: its path, or `$<fn>`.
func messageKey(s *slot) string {
	switch {
	case s.fn != nil:
		return dollar + s.fn.Name
	case s.src == nil || s.src.Inline || len(s.src.WirePath) == 0:
		return s.field
	}
	return strings.Join(s.src.WirePath, dot)
}

// refCell is where a resolved ref reads its key and writes its entry: a slot's members or one
// cell of a lookup's tables; ok is "" when the key is always there, dstOK when dst has no mark.
type refCell struct {
	keys, ok, dst, dstOK string
	list                 bool
	elem                 ir.Type
}

// findIn is the Find of the target's rows, in the snapshot or in the holder's own container.
func (g *gen) findIn(target *ir.Value, snapshot bool) string {
	find := g.lc.Ctx + dot
	if snapshot {
		find += g.names.ValueStore(target) + dot
	}
	return find + ir.GoRows + dot + ir.GoFind
}

// resolveSlot points a resolved slot at its entries.
func (g *gen) resolveSlot(b *strings.Builder, s *slot, target *ir.Value, snapshot bool) {
	if target == nil {
		return
	}
	out := g.lc.Out + dot
	c := refCell{keys: out + s.KeyStore, dst: out + s.Store, list: s.List, elem: s.Ref.Elem}
	if s.Optional {
		c.ok = out + s.OKStore
	}
	g.resolveRef(b, c, g.findIn(target, snapshot), g.keyLoc(messageKey(s)))
}

// resolveCells resolves every cell of a lookup's key table into its entry table, a failure named by its domain keys (CODEGEN.md §5.10).
func (g *gen) resolveCells(b *strings.Builder, f *finiteMethod, target *ir.Value, snapshot bool) {
	if target == nil {
		return
	}
	keys, dst, loc := g.lc.Out+dot+f.res.KeyStore, g.lc.Out+dot+f.store, g.keyLoc(dollar+f.fn.Name)
	for _, p := range f.fn.Params {
		i, k := g.temp(tempIndex), g.temp(tempKey)
		fmt.Fprintf(b, domainLoopFormat, i, k, strings.Join(g.domainKeys(p.Type), listSep))
		keys, dst, loc = keys+lbracket+i+rbracket, dst+lbracket+i+rbracket, loc.dot().arg(k)
	}
	c := refCell{keys: keys, dst: dst, list: f.res.List, elem: f.res.Ref.Elem}
	if f.res.Optional {
		c.keys, c.ok = keys+pairValue, keys+pairOK
	}
	if f.pair {
		c.dst, c.dstOK = dst+pairValue, dst+pairOK
	}
	g.resolveRef(b, c, g.findIn(target, snapshot), loc)
	b.WriteString(strings.Repeat(closeBrace, len(f.dims)))
}

// resolveRef points dst at the entries its keys name; a key naming no entry fails the load
// with the row, the field and the key (log-2026-09-24, gen/cpp round 3).
func (g *gen) resolveRef(b *strings.Builder, c refCell, find string, loc location) {
	if c.ok != "" {
		fmt.Fprintf(b, ifOpenFormat, c.ok)
	}
	if c.list {
		v, i, e, ok := g.temp(tempValue), g.temp(tempIndex), g.temp(tempEntry), g.temp(tempOK)
		fmt.Fprintf(b, resolveListFormat, v, g.typeName(c.elem), c.keys, i, e, ok, find,
			g.errAt(loc.index(i), noEntryText, c.keys+atCall+i+rparen), c.dst, g.rt())
	} else {
		e, ok := g.temp(tempEntry), g.temp(tempOK)
		fmt.Fprintf(b, resolveOneFormat, e, ok, find, c.keys, g.errAt(loc, noEntryText, c.keys), c.dst)
	}
	if c.dstOK != "" {
		fmt.Fprintf(b, markFormat, c.dstOK)
	}
	if c.ok != "" {
		b.WriteString(closeBrace)
	}
}

// walkSlot resolves the classes a field or stored result holds; an inline variant shares
// its parent's path.
func (g *gen) walkSlot(b *strings.Builder, s *slot) {
	inline := s.src != nil && s.src.Inline
	g.walkValue(b, s.T, g.lc.Out+dot+s.Store, g.keyLoc(messageKey(s)), inline)
}

// walkValue resolves the classes a value holds: directly, or through a list's elements.
func (g *gen) walkValue(b *strings.Builder, t ir.TypeRef, expr string, loc location, inline bool) {
	lc := g.lc
	if t.Kind == types.List {
		i := g.temp(tempIndex)
		fmt.Fprintf(b, walkListFormat, i, expr)
		g.walkValue(b, g.sub(t.Elem), expr+atCall+i+rparen, loc.index(i), false)
		b.WriteString(closeBrace)
		return
	}
	prefix := g.locExpr(loc.dot())
	if inline {
		prefix = lc.Path
	}
	fmt.Fprintf(b, walkOneFormat, expr, lc.Err, g.resolveFunc(t.Named), lc.Name, prefix, lc.Ctx)
}

// walkPairs resolves a pairs field's elements' refs, named by their slot keys (WIRE.md §4.1, §5.14).
func (g *gen) walkPairs(b *strings.Builder, s *slot, snapshot bool) {
	rec, ok := g.sub(s.T.Elem).Named.(*ir.Record)
	if !ok || len(g.bodyOf(rec).slots) < pairFields {
		return
	}
	eb := g.bodyOf(rec)
	if !slices.ContainsFunc(eb.slots[:pairFields], func(es *slot) bool { return es.Resolved }) {
		return
	}
	i, e := g.temp(tempIndex), g.temp(tempElem)
	fmt.Fprintf(b, pairsWalkFormat, i, g.lc.Out+dot+s.Store, e)
	for k, es := range eb.slots[:pairFields] {
		target := g.slotTarget(es)
		if target == nil {
			continue
		}
		key := g.temp(tempKey)
		fmt.Fprintf(b, slotKeyFormat, key, strings.Join(g.slotKeys(s.src.Pairs.Keys[k], s.src.Pairs.Slots), listSep), i)
		loc := g.root().arg(key)
		c := refCell{keys: e + dot + es.KeyStore, dst: e + dot + es.Store, list: es.List, elem: es.Ref.Elem}
		if es.Optional {
			c.ok = e + dot + es.OKStore
		}
		g.resolveRef(b, c, g.findIn(target, snapshot), loc)
	}
	b.WriteString(closeBrace)
}
