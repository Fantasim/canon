package edit

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// fieldPlace is the insertion d.out[at] of a new member into holder, the object at key path
// prefix of one stating rec.
type fieldPlace struct {
	at     int
	holder *jsonsrc.Node
	rec    *value.Record
	prefix []string
}

// settle ends the comparison of rec, stated by n, its edits from d.out[from], its new members
// from d.places[placed]: each new member placed among the members that stay, then each object a
// `@json(path:)` creates that they empty removed (FORMATTER.md 14.2, WIRE.md 5.5.3, DECISIONS 471).
func (d *jsonDiff) settle(n *jsonsrc.Node, rec *value.Record, from, placed int) {
	prefixes := pathPrefixes(rec)
	if len(d.places) == placed && len(prefixes) == 0 {
		return
	}
	gone := d.removedSince(from)
	for _, p := range d.places[placed:] {
		d.place(p, gone)
	}
	d.places = d.places[:placed]
	for _, p := range prefixes {
		d.dropEmptied(n, p, from, gone)
	}
}

// removedSince are the pointers of the members the edits from d.out[from] remove.
func (d *jsonDiff) removedSince(from int) map[string]bool {
	gone := map[string]bool{}
	for _, e := range d.out[from:] {
		if e.e.Kind == jsonsrc.Remove {
			gone[e.e.Pointer] = true
		}
	}
	return gone
}

// place moves insertion p, into an object holding an unknown key, right after the nearest earlier
// declared key that stays and past the removed and kept keys after it, else before the first later
// declared key that stays, else to the end: the kept key comes first (FORMATTER.md 14.2, 471).
func (d *jsonDiff) place(p fieldPlace, gone map[string]bool) {
	if !slices.ContainsFunc(p.holder.Members, func(m jsonsrc.Member) bool { return !claimAt(p.rec, p.prefix, m.Key).claimed }) {
		return
	}
	e := &d.out[p.at]
	declared := declaredKeys(p.rec, p.prefix)
	at := slices.Index(declared, e.e.Key)
	if at < 0 {
		return
	}
	stays := func(key string) int {
		j := objMember(p.holder, key)
		if j >= 0 && gone[p.holder.Members[j].Value.Pointer()] {
			return -1
		}
		return j
	}
	to := len(p.holder.Members)
	if j := firstOf(slices.Backward(declared[:at]), stays); j >= 0 {
		to = p.pastKept(j+1, gone)
	} else if j := firstOf(slices.All(declared[at+1:]), stays); j >= 0 {
		to = j
	}
	e.e.At, e.anchor = to, itemAnchor(p.holder, to)
}

// firstOf is the first index at that keys gives, -1 for none.
func firstOf(keys func(func(int, string) bool), at func(string) int) int {
	for _, key := range keys {
		if j := at(key); j >= 0 {
			return j
		}
	}
	return -1
}

// pastKept is the index of the first member of p's holder from at on that stays and a field of
// p's record claims.
func (p fieldPlace) pastKept(at int, gone map[string]bool) int {
	for at < len(p.holder.Members) {
		m := p.holder.Members[at]
		if !gone[m.Value.Pointer()] && claimAt(p.rec, p.prefix, m.Key).claimed {
			break
		}
		at++
	}
	return at
}

// dropEmptied removes the object at key path p under n, one a `@json(path:)` creates, when the
// edits from d.out[from] remove each of its members and add none: one removal in place of
// theirs. It stays while it holds a member, an unknown key included (WIRE.md 5.5.3; ruling).
func (d *jsonDiff) dropEmptied(n *jsonsrc.Node, p []string, from int, gone map[string]bool) {
	obj, holder, _ := memberAt(n, p)
	if obj == nil || obj.Kind != jsonsrc.Object || len(obj.Members) == 0 {
		return
	}
	ptr := obj.Pointer()
	for _, m := range obj.Members {
		if !gone[m.Value.Pointer()] {
			return
		}
	}
	inside := ptr + pointerSep
	if slices.ContainsFunc(d.out[from:], func(e jsonEdit) bool { return e.e.Kind == jsonsrc.Insert && e.e.Pointer == ptr }) {
		return
	}
	kept := slices.DeleteFunc(d.out[from:], func(e jsonEdit) bool { return strings.HasPrefix(e.e.Pointer, inside) })
	d.out = d.out[:from+len(kept)]
	d.remove(obj, memberStart(holder, p))
	gone[ptr] = true
}

// pathPrefixes are the key paths of the objects the `@json(path:)` fields of rec and of the inline
// cases it holds create, the deepest first (WIRE.md 5.5.3).
func pathPrefixes(rec *value.Record) [][]string {
	out := addPrefixes(nil, rec)
	slices.SortStableFunc(out, func(x, y []string) int { return cmp.Compare(len(y), len(x)) })
	return out
}

// addPrefixes is out with pathPrefixes's key paths of rec added once each, in field order.
func addPrefixes(out [][]string, rec *value.Record) [][]string {
	for j, f := range fieldsOf(rec.T) {
		if c, ok := fieldValue(rec, j).(*value.Record); ok && c != nil && f.Inline {
			out = addPrefixes(out, c)
		}
		if f.Inline || f.Pairs != nil || f.Input != nil {
			continue
		}
		for k := len(f.WirePath) - 1; k >= 1; k-- {
			if !slices.ContainsFunc(out, func(q []string) bool { return slices.Equal(q, f.WirePath[:k]) }) {
				out = append(out, f.WirePath[:k])
			}
		}
	}
	return out
}

// fieldValue is field j of rec, nil past its fields.
func fieldValue(rec *value.Record, j int) value.Value {
	if j >= len(rec.Fields) {
		return nil
	}
	return rec.Fields[j]
}

// declaredKeys are the keys at prefix of an object stating rec, in declaration order: a path's
// key where its first field stands, a pairs field's slot keys, an inline case's tag and keys
// (FORMATTER.md 14.2 "wire names"; WIRE.md 5.5.3, 5.6, 5.14).
func declaredKeys(rec *value.Record, prefix []string) []string {
	var out []string
	for j, f := range fieldsOf(rec.T) {
		for _, k := range fieldKeys(fieldValue(rec, j), f, prefix) {
			if !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	return out
}

// fieldKeys are the keys field f, holding v (nil when unknown), writes at prefix, in order.
func fieldKeys(v value.Value, f *types.Field, prefix []string) []string {
	p := f.WirePath
	switch {
	case f.Input != nil:
	case f.Pairs != nil:
		if len(prefix) == 0 {
			return slotKeys(f.Pairs)
		}
	case f.Inline:
		return inlineKeys(v, f.Type, prefix)
	case len(p) > len(prefix) && slices.Equal(p[:len(prefix)], prefix):
		return p[len(prefix) : len(prefix)+1]
	}
	return nil
}

// inlineKeys are the keys an inline field of type t holding v writes at prefix: its variant's tag,
// then its case's keys, every case's when v is unknown (WIRE.md 5.6).
func inlineKeys(v value.Value, t types.Type, prefix []string) []string {
	var out []string
	vt, isVariant := t.Base().(*types.VariantType)
	if isVariant && len(prefix) == 0 {
		out = append(out, vt.Tag)
	}
	if c, ok := v.(*value.Record); ok && c != nil {
		return append(out, declaredKeys(c, prefix)...)
	}
	if !isVariant {
		return out
	}
	for _, k := range vt.Cases {
		out = append(out, declaredKeys(&value.Record{T: k, Fields: make([]value.Value, len(k.Fields))}, prefix)...)
	}
	return out
}

// slotKeys are the keys of p's slots in slot order, each slot's key then value (WIRE.md 5.14).
func slotKeys(p *types.Pairs) []string {
	var out []string
	for i := range p.Slots {
		for _, tmpl := range p.Keys {
			out = append(out, strings.Replace(tmpl, slotHole, strconv.Itoa(i), 1))
		}
	}
	return out
}
