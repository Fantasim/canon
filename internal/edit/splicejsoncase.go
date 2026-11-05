package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

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
		cn, err := d.caseWire(cv, &plain)
		if err != nil || cn.Kind != jsonsrc.Object {
			return nil, &NotEditableError{Reason: ReasonComputed}
		}
		out.Members = append(out.Members, cn.Members...)
	}
	return &out, nil
}

// caseWire is v, an inline field's value, in its own object: a record's inline fields with their
// tags (WIRE.md 5.6, 5.8).
func (d *jsonDiff) caseWire(v value.Value, f *types.Field) (*jsonsrc.Node, error) {
	if rec, ok := v.(*value.Record); ok && rec != nil {
		return d.recordWire(rec, f)
	}
	if v == nil {
		return nil, &NotEditableError{Reason: ReasonComputed}
	}
	return d.a.wireNode(v, f)
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
