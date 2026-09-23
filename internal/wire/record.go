package wire

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// record writes r's members after those obj already holds ($id, $retired).
func (e *encoder) record(r *value.Record, obj *node) (*node, error) {
	if err := e.members(obj, r); err != nil {
		return nil, err
	}
	return obj, nil
}

// members writes a case's tag, the fields, then the `$` keys into obj (WIRE.md §5.5.1).
func (e *encoder) members(obj *node, r *value.Record) error {
	fields, c, err := shape(r)
	if err != nil {
		return err
	}
	if c != nil {
		obj.add(c.Variant.Tag, stringNode(c.Wire))
	}
	for i, f := range fields {
		if err := e.field(obj, f, r.Fields[i]); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return e.dollars(obj, r)
}

// shape is the declared fields of a record value, and its case when it is a variant's.
func shape(r *value.Record) ([]*types.Field, *types.CaseType, error) {
	var fields []*types.Field
	var c *types.CaseType
	found := r.T != nil
	if found {
		switch d := r.T.Base().(type) {
		case *types.RecordType:
			fields = d.Fields
		case *types.AppliedRecord:
			fields = d.Rec.Fields
		case *types.CaseType:
			fields, c = d.Fields, d
		default:
			found = false
		}
	}
	if !found || len(fields) != len(r.Fields) {
		return nil, nil, fmt.Errorf("%w: record value of %v", ErrShape, r.T)
	}
	return fields, c, nil
}

// field writes one field: an inline case, pairs slots, or the value at its key path.
func (e *encoder) field(obj *node, f *types.Field, v value.Value) error {
	switch {
	case f.Input != nil:
		return nil
	case f.Inline:
		return e.inline(obj, v)
	case f.Pairs != nil:
		return e.pairs(obj, f, v)
	case len(f.WirePath) == 0:
		return fmt.Errorf("%w: no wire path", ErrShape)
	}
	n, err := e.fieldValue(f, v)
	if err != nil {
		return err
	}
	return obj.put(f.WirePath, n)
}

func (e *encoder) inline(obj *node, v value.Value) error {
	r, ok := v.(*value.Record)
	if !ok {
		return fmt.Errorf("%w: inline %T", ErrShape, v)
	}
	if _, c, err := shape(r); err != nil || c == nil {
		return fmt.Errorf("%w: inline %v", ErrShape, r.T)
	}
	return e.members(obj, r)
}

// pairs writes element i as the keys k(i) then v(i), empty slots unwritten (WIRE.md §5.14).
func (e *encoder) pairs(obj *node, f *types.Field, v value.Value) error {
	l, ok := v.(*value.List)
	if !ok || len(l.Elems) > f.Pairs.Slots {
		return fmt.Errorf("%w: pairs %T", ErrShape, v)
	}
	for i, el := range l.Elems {
		r, ok := el.(*value.Record)
		if !ok {
			return fmt.Errorf("%w: pair %T", ErrShape, el)
		}
		fields, c, err := shape(r)
		if err != nil || c != nil || len(fields) != len(f.Pairs.Keys) {
			return fmt.Errorf("%w: pair of %v", ErrShape, r.T)
		}
		for j, tmpl := range f.Pairs.Keys {
			n, err := e.fieldValue(fields[j], r.Fields[j])
			if err != nil {
				return err
			}
			obj.add(strings.Replace(tmpl, pairsIndex, strconv.Itoa(i), 1), n)
		}
	}
	return nil
}

// fieldValue encodes a field's value with its none marker, unit and encoding (§5.1–§5.4).
func (e *encoder) fieldValue(f *types.Field, v value.Value) (*node, error) {
	if _, none := v.(*value.None); none {
		if f.NoneWire != nil {
			return scalar(f.NoneWire), nil
		}
		return text(textNull), nil
	}
	var n *node
	var err error
	if l, ok := v.(*value.List); ok && f.Enc == types.EncBits {
		n, err = bits(l)
	} else {
		n, err = e.value(v, scope{unit: f.Unit, asInt: f.Enc == types.EncInt})
	}
	if err == nil && f.NoneWire != nil && sameJSON(n, f.NoneWire) {
		return nil, fmt.Errorf("%w: %s", ErrNoneMarker, f.NoneWire)
	}
	return n, err
}

// bits is the OR of the members' codes; a code already set is a repeated member.
func bits(l *value.List) (*node, error) {
	var mask int64
	for _, el := range l.Elems {
		m, ok := el.(*value.Member)
		if !ok || m.Index < 0 || m.Index >= len(m.Enum.Members) {
			return nil, fmt.Errorf("%w: bits element %T", ErrShape, el)
		}
		code := m.Enum.Members[m.Index].Code
		if mask&code != 0 {
			return nil, fmt.Errorf("%w: %s", ErrBitsRepeated, m.Enum.Members[m.Index].Name)
		}
		mask |= code
	}
	return scalar(strconv.AppendInt(nil, mask, decimalBase)), nil
}

// dollars writes the `$` keys of r's export fns, in the order Methods gives (§5.11).
func (e *encoder) dollars(obj *node, r *value.Record) error {
	if e.methods == nil {
		return nil
	}
	for _, fn := range e.methods(r) {
		n, err := e.fn(fn)
		if err != nil {
			return fmt.Errorf("%s: %w", fn.Name, err)
		}
		obj.add(keyDollar+fn.Name, n)
	}
	return nil
}

// fn is a precomputed result, or nested objects, one level per finite parameter (§5.11).
func (e *encoder) fn(fn Fn) (*node, error) {
	if len(fn.Domains) == 0 {
		return e.value(fn.Result, scope{})
	}
	cells := 1
	for _, d := range fn.Domains {
		cells *= len(d)
	}
	if cells != len(fn.Cells) {
		return nil, fmt.Errorf("%w: %d cells for %d arguments", ErrShape, len(fn.Cells), cells)
	}
	return e.lookup(fn.Domains, fn.Cells)
}

func (e *encoder) lookup(domains [][]value.Value, cells []value.Value) (*node, error) {
	if len(domains) == 0 {
		return e.value(cells[0], scope{})
	}
	obj := objectNode()
	if len(domains[0]) == 0 {
		return obj, nil
	}
	stride := len(cells) / len(domains[0])
	seen := make(map[string]bool, len(domains[0]))
	for i, arg := range domains[0] {
		key, err := keyText(arg, seen)
		if err != nil {
			return nil, err
		}
		sub, err := e.lookup(domains[1:], cells[i*stride:(i+1)*stride])
		if err != nil {
			return nil, err
		}
		obj.add(key, sub)
	}
	return obj, nil
}
