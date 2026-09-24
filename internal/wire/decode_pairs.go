package wire

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// slot is slot i of a pairs field: its two keys and the members found under them.
type slot struct {
	i      int
	keys   [pairKeys]string
	found  [pairKeys]*jsonsrc.Member
	filled bool
}

// pairs reads a @json(pairs:) field: its filled slots, contiguous from 0 (WIRE.md §5.14).
func (r *run) pairs(o *object, f *types.Field, i int, rv *value.Record) bool {
	elem, fields, ok := pairShape(f)
	if !ok {
		r.misuse(ErrNoWireType, f.Type)
		return false
	}
	slots := slotsOf(o, f.Pairs)
	l := &value.List{T: f.Type, P: prov(o.n)}
	rv.Set[i] = len(slots) > 0
	next, empty := 0, -1
	ok = true
	for _, s := range slots {
		if empty < 0 && s.i > next {
			empty = next
		}
		next = s.i + 1
		if !r.slotValid(s, f, empty) {
			ok = false
			continue
		}
		if e := r.pair(s, elem, fields); e != nil {
			l.Elems = append(l.Elems, e)
			continue
		}
		ok = false
	}
	rv.Fields[i] = l
	return ok
}

// slotMatch is a member whose key is template k of slot n.
type slotMatch struct {
	n, k   int
	member int
}

// slotsOf claims the members whose keys are a template with a slot below the bound, grouped
// by slot in slot order: one pass over the members, whatever the bound.
func slotsOf(o *object, p *types.Pairs) []slot {
	var matches []slotMatch
	for m := range o.n.Members {
		for k, tmpl := range p.Keys {
			if n, ok := slotIndex(o.n.Members[m].Key, tmpl, p.Slots); ok {
				matches = append(matches, slotMatch{n: n, k: k, member: m})
				o.claims[o.n][m] = true
				break
			}
		}
	}
	slices.SortStableFunc(matches, func(a, b slotMatch) int { return cmp.Compare(a.n, b.n) })
	var out []slot
	for _, mt := range matches {
		if len(out) == 0 || out[len(out)-1].i != mt.n {
			out = append(out, newSlot(p, mt.n))
		}
		out[len(out)-1].found[mt.k] = &o.n.Members[mt.member]
	}
	for j := range out {
		f := out[j].found
		out[j].filled = f[0] != nil && f[1] != nil && f[0].Value.Kind != jsonsrc.Null && f[1].Value.Kind != jsonsrc.Null
	}
	return out
}

func newSlot(p *types.Pairs, n int) slot {
	s := slot{i: n}
	for k, tmpl := range p.Keys {
		s.keys[k] = strings.Replace(tmpl, pairsIndex, strconv.Itoa(n), 1)
	}
	return s
}

// slotIndex is the slot whose key under tmpl is key: its {i} a decimal without leading zero,
// below the number of slots.
func slotIndex(key, tmpl string, slots int) (int, bool) {
	prefix, suffix, _ := strings.Cut(tmpl, pairsIndex)
	mid, ok := strings.CutPrefix(key, prefix)
	if !ok {
		return 0, false
	}
	mid, ok = strings.CutSuffix(mid, suffix)
	if !ok || !slotPattern.MatchString(mid) {
		return 0, false
	}
	n, err := strconv.Atoi(mid)
	return n, err == nil && n < slots
}

// pairShape is a pairs field's element type and its two fields (WIRE.md §4.1).
func pairShape(f *types.Field) (types.Type, []*types.Field, bool) {
	lt, ok := f.Type.Base().(*types.ListType)
	if !ok {
		return nil, nil, false
	}
	rt, ok := lt.Elem.Base().(*types.RecordType)
	if !ok || len(rt.Fields) != pairKeys {
		return nil, nil, false
	}
	return lt.Elem, rt.Fields, true
}

// slotValid is E7117 for a slot with one key, a null, or filled after an empty one.
func (r *run) slotValid(s slot, f *types.Field, empty int) bool {
	first := s.found[0]
	if first == nil {
		first = s.found[1]
	}
	slotNo := int64(s.i)
	switch {
	case s.found[0] == nil:
		r.report(diag.E7117.AtKey(first.KeySpan, slotNo, f.Name, s.keys[1], s.keys[0]), first.Value)
	case s.found[1] == nil:
		r.report(diag.E7117.AtKey(first.KeySpan, slotNo, f.Name, s.keys[0], s.keys[1]), first.Value)
	case !s.filled:
		r.report(diag.E7117.AtNull(first.KeySpan, slotNo, f.Name), first.Value)
	case empty >= 0:
		r.report(diag.E7117.AtGap(first.KeySpan, slotNo, f.Name, int64(empty)), first.Value)
	default:
		return true
	}
	return false
}

// pair is the element a filled slot holds: each value read as its field, unit and int applied.
func (r *run) pair(s slot, elem types.Type, fields []*types.Field) value.Value {
	pv := &value.Record{T: elem, Fields: make([]value.Value, pairKeys), Set: []bool{true, true}, P: prov(s.found[0].Value)}
	ok := true
	for k, f := range fields {
		pv.Fields[k] = r.value(Selection{Node: s.found[k].Value}, f.Type, fieldScope(f, &frame{rec: pv}))
		ok = ok && pv.Fields[k] != nil
	}
	if !ok {
		return nil
	}
	return pv
}
