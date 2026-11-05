package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// rewriteKeeping writes v's wire over object n, which states was, as field f, keeping in place
// each member no field of was or v and no case of their variants claims: data `partial` kept (M8,
// FORMATTER.md 14.1). A case change drops the old case's keys, which are Canon's (ruling).
func (d *jsonDiff) rewriteKeeping(n *jsonsrc.Node, was, v *value.Record, f *types.Field) error {
	node, err := d.recordWire(v, f)
	if err != nil || node.Kind != jsonsrc.Object {
		return &NotEditableError{Reason: ReasonComputed} // no object beside the kept keys decodes to it
	}
	out := *node
	out.Members = keptInPlace(n.Members, node.Members, func(key string) bool { return !ownedKey(was, key) && !ownedKey(v, key) })
	d.setNode(n, &out)
	return nil
}

// keptInPlace is members with each of old that kept reports put back in place: after the nearest
// member before it in old that the result holds, else first.
func keptInPlace(old, members []jsonsrc.Member, kept func(string) bool) []jsonsrc.Member {
	out := slices.Clone(members)
	for i, mem := range old {
		if !kept(mem.Key) {
			continue
		}
		at := 0
		for _, before := range slices.Backward(old[:i]) {
			if k := keyedAt(out, before.Key); k >= 0 {
				at = k + 1
				break
			}
		}
		out = slices.Insert(out, at, mem)
	}
	return out
}

// keyedAt is the index of the member keyed key in members, -1 for none.
func keyedAt(members []jsonsrc.Member, key string) int {
	return slices.IndexFunc(members, func(m jsonsrc.Member) bool { return m.Key == key })
}

// recordWire is v's object at field f's place, each inline variant written with its tag, from its
// default when v leaves it out (WIRE.md 5.8), its members in declaration order (FMT-02).
func (d *jsonDiff) recordWire(v *value.Record, f *types.Field) (*jsonsrc.Node, error) {
	fields := fieldsOf(v.T)
	bare := withField(v, -1, nil)
	var inline []int
	for j, fj := range fields {
		if fj.Inline && j < len(bare.Fields) {
			inline, bare.Fields[j], bare.Set[j] = append(inline, j), nil, false
		}
	}
	node, err := d.a.wireNode(bare, f)
	if err != nil || len(inline) == 0 {
		return node, err
	}
	return d.withInline(node, v, inline)
}

// withInline is node, record v's wire without its inline fields, with each one's members put at
// its place in declaration order, after the members no field writes (a case's tag, WIRE.md 5.8).
func (d *jsonDiff) withInline(node *jsonsrc.Node, v *value.Record, inline []int) (*jsonsrc.Node, error) {
	fields := fieldsOf(v.T)
	out := *node
	out.Members = slices.DeleteFunc(slices.Clone(node.Members), func(m jsonsrc.Member) bool { return fieldKey(fields, m.Key) })
	for j, fj := range fields {
		if !slices.Contains(inline, j) {
			out.Members = append(out.Members, membersFor(node, fj)...)
			continue
		}
		cv := v.Fields[j]
		if cv == nil {
			cv, _ = d.a.defaultOf(v, j)
		}
		plain := *fj
		plain.Inline = false
		cn, err := d.a.wireNode(cv, &plain)
		if cv == nil || err != nil || cn.Kind != jsonsrc.Object {
			return nil, &NotEditableError{Reason: ReasonComputed}
		}
		out.Members = append(out.Members, cn.Members...)
	}
	return &out, nil
}

// fieldKey reports key the first key of a non-inline field among fields.
func fieldKey(fields []*types.Field, key string) bool {
	return slices.ContainsFunc(fields, func(f *types.Field) bool { return !f.Inline && len(f.WirePath) > 0 && f.WirePath[0] == key })
}

// membersFor are the members of node a non-inline field f writes: its first key's.
func membersFor(node *jsonsrc.Node, f *types.Field) []jsonsrc.Member {
	if len(f.WirePath) == 0 {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(node.Members), func(m jsonsrc.Member) bool { return m.Key != f.WirePath[0] })
}

// ownedKey reports key claimed in an object stating rec, or by any case of a variant rec is a case
// of or an inline field of it holds (WIRE.md 5.5, 5.8).
func ownedKey(rec *value.Record, key string) bool {
	if _, claimed := claimedBy(rec, key); claimed || variantClaims(rec.T, key) {
		return true
	}
	for j, f := range fieldsOf(rec.T) {
		if j >= len(rec.Fields) || !f.Inline {
			continue
		}
		if c, ok := rec.Fields[j].(*value.Record); ok && c != nil && variantClaims(c.T, key) {
			return true
		}
	}
	return false
}

// variantClaims reports key the tag of the variant t is a case of, or a key one of its cases claims.
func variantClaims(t types.Type, key string) bool {
	c, ok := t.Base().(*types.CaseType)
	if !ok {
		return false
	}
	if key == c.Variant.Tag {
		return true
	}
	return slices.ContainsFunc(c.Variant.Cases, func(k *types.CaseType) bool {
		return slices.ContainsFunc(k.Fields, func(f *types.Field) bool {
			return f.Pairs != nil || len(f.WirePath) > 0 && f.WirePath[0] == key
		})
	})
}
