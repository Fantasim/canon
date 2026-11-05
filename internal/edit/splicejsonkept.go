package edit

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// dropMember leaves field i of nw, stated by member m in holder, to its default (API.md E6, Reset).
// An object holding keys no field claims, which `partial` kept (M8), keeps them, and the default's
// members it would not decode to are written beside them, as W9 writes a default (ruling).
func (d *jsonDiff) dropMember(old, nw *value.Record, i int, m, holder *jsonsrc.Node) error {
	f := fieldsOf(nw.T)[i]
	if !unclaimedIn(m, old.Fields[i]) {
		d.remove(m, memberStart(holder, f.WirePath))
		return nil
	}
	dv, ok := d.a.defaultOf(nw, i)
	def, isRec := dv.(*value.Record)
	was, wasRec := old.Fields[i].(*value.Record)
	switch {
	case !ok || !isRec || !wasRec:
		return &NotEditableError{Reason: ReasonComputed} // no object beside the kept keys decodes to it
	case !sameShape(was.T, def.T):
		return d.swapCase(m, swap{old: was, nw: wiredDefault(d.a, def)}, f) // another case: its keys go
	}
	if err := d.value(was, wiredDefault(d.a, def), m, f); err != nil {
		return &NotEditableError{Reason: ReasonComputed}
	}
	return nil
}

// wiredDefault is the record def, a default, with only the fields its record's defaults do not give
// marked written: the members an object must hold to decode to def (WIRE.md 5.4).
func wiredDefault(a *applier, def *value.Record) *value.Record {
	out := withField(def, -1, nil)
	for j := range out.Fields {
		vd, _ := a.defaultVerdict(depFrame{rec: def, a: a}, j)
		out.Set[j] = out.Fields[j] != nil && vd != vEqual
	}
	return out
}

// unclaimedIn reports n an object stating v, a record, that holds a key no field of v's type
// claims, in it, in an object a field of it claims or in one a `@json(path:)` creates (WIRE.md
// 5.5, 5.5.3, 6.4).
func unclaimedIn(n *jsonsrc.Node, v value.Value) bool {
	rec, ok := v.(*value.Record)
	if n == nil || n.Kind != jsonsrc.Object || !ok || rec == nil {
		return false
	}
	return unclaimedAt(n, rec, nil)
}

// unclaimedAt is unclaimedIn for n, the object at key path prefix of an object stating rec.
func unclaimedAt(n *jsonsrc.Node, rec *value.Record, prefix []string) bool {
	for _, mem := range n.Members {
		c := claimAt(rec, prefix, mem.Key)
		switch {
		case !c.claimed:
			return true
		case c.under && unclaimedAt(mem.Value, rec, append(slices.Clone(prefix), mem.Key)):
			return true
		case !c.under && unclaimedIn(mem.Value, c.v):
			return true
		}
	}
	return false
}

// claim is how a key is claimed in an object stating a record: by a field holding v, or as an
// object a `@json(path:)` creates (under).
type claim struct {
	v              value.Value
	claimed, under bool
}

// claimAt is how key is claimed in the object at key path prefix of an object stating rec: a
// field's key, an inline case's tag or key, a pairs field's slot key (WIRE.md 5.5, 5.8, 5.14).
func claimAt(rec *value.Record, prefix []string, key string) claim {
	if c, isCase := rec.T.Base().(*types.CaseType); isCase && len(prefix) == 0 && key == c.Variant.Tag {
		return claim{claimed: true}
	}
	for j, f := range fieldsOf(rec.T) {
		if j >= len(rec.Fields) {
			continue
		}
		if c := fieldClaim(f, rec.Fields[j], prefix, key); c.claimed {
			return c
		}
	}
	return claim{}
}

// fieldClaim is claimAt for field f, holding v.
func fieldClaim(f *types.Field, v value.Value, prefix []string, key string) claim {
	c, isRec := v.(*value.Record)
	p := f.WirePath
	switch {
	case f.Pairs != nil:
		return claim{claimed: len(prefix) == 0 && slotKey(f.Pairs, key)}
	case f.Inline && isRec && c != nil:
		return claimAt(c, prefix, key)
	case f.Inline || f.Input != nil || !keyAt(p, prefix, key):
		return claim{}
	case len(p) == len(prefix)+1:
		return claim{v: v, claimed: true}
	}
	return claim{claimed: true, under: true} // an object @json(path:) creates (LOD-09)
}

// keyAt reports key path p holding key right after prefix.
func keyAt(p, prefix []string, key string) bool {
	return len(p) > len(prefix) && p[len(prefix)] == key && slices.Equal(p[:len(prefix)], prefix)
}

// slotKey reports key one of p's slot keys: a template with its {i} a decimal without a leading
// zero below the number of slots (WIRE.md 5.14); a key past the last slot is no field's. It reads
// keys as the decoder's slotIndex (internal/wire/decode_pairs.go) does.
func slotKey(p *types.Pairs, key string) bool {
	return slices.ContainsFunc(p.Keys[:], func(tmpl string) bool {
		before, after, _ := strings.Cut(tmpl, slotHole)
		mid, isBefore := strings.CutPrefix(key, before)
		mid, isAfter := strings.CutSuffix(mid, after)
		n, err := strconv.Atoi(mid)
		return isBefore && isAfter && err == nil && n >= 0 && n < p.Slots && strconv.Itoa(n) == mid
	})
}
