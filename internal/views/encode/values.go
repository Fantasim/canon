package encode

import (
	"bytes"
	"encoding/json"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Value is v in the view model's value encoding (VIEWMODEL.md J10), each number in its
// canonical text (WIRE.md 7.2); nil, an input field's missing value, is null.
func Value(v value.Value) json.RawMessage {
	var b bytes.Buffer
	write(&b, v)
	return b.Bytes()
}

// write is v's encoding; a value without one (a pair, a range) is null.
func write(b *bytes.Buffer, v value.Value) {
	switch x := v.(type) {
	case *value.Bool:
		b.WriteString(x.CanonText())
	case *value.Int:
		writeInt(b, x.V)
	case *value.Dur:
		writeInt(b, x.Ms)
	case *value.Float:
		writeFloat(b, x)
	case *value.Str, *value.Member, *value.CaseKind, *value.Symbol:
		writeString(b, x.CanonText())
	case *value.Ref:
		writeKey(b, x.Key)
	case *value.List:
		writeList(b, x.Elems)
	case *value.Map:
		writeMap(b, x)
	case *value.Table:
		writeTable(b, x)
	case *value.Record:
		writeRecord(b, x)
	default:
		b.WriteString(valueNull)
	}
}

// writeInt is an integer, a decimal string beyond ±(2^53−1) (J10).
func writeInt(b *bytes.Buffer, i int64) {
	n := vm.Int(i)
	if n.Quoted {
		writeString(b, n.Text)
		return
	}
	b.WriteString(n.Text)
}

func writeFloat(b *bytes.Buffer, f *value.Float) {
	bits := types.FloatType.Bits
	if basic, ok := f.T.Base().(types.Basic); ok {
		bits = basic.Bits
	}
	b.WriteString(types.FloatText(f.V, bits))
}

func writeString(b *bytes.Buffer, s string) {
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	if enc.Encode(s) == nil {
		b.Truncate(b.Len() - 1) // Encode's trailing newline
	}
}

// writeKey is a ref's or an entry's key: a string, or a number for an integer key.
func writeKey(b *bytes.Buffer, k value.Key) {
	if k.IsInt {
		writeInt(b, k.I)
		return
	}
	writeString(b, k.S)
}

func writeList(b *bytes.Buffer, elems []value.Value) {
	b.WriteByte(openArray)
	for i, e := range elems {
		if i > 0 {
			b.WriteByte(comma)
		}
		write(b, e)
	}
	b.WriteByte(closeArray)
}

// writeMap is an array of [key, value] pairs in map order (J10).
func writeMap(b *bytes.Buffer, m *value.Map) {
	b.WriteByte(openArray)
	for i := range m.Keys {
		if i > 0 {
			b.WriteByte(comma)
		}
		writeList(b, []value.Value{m.Keys[i], m.Vals[i]})
	}
	b.WriteByte(closeArray)
}

// writeTable is a table as a map from its keys to its entries.
func writeTable(b *bytes.Buffer, t *value.Table) {
	b.WriteByte(openArray)
	for i, e := range t.Entries {
		if i > 0 {
			b.WriteByte(comma)
		}
		b.WriteByte(openArray)
		writeKey(b, e.Ident.Key)
		b.WriteByte(comma)
		writeRecord(b, e)
		b.WriteByte(closeArray)
	}
	b.WriteByte(closeArray)
}

// writeRecord is an object of Canon field names in declaration order, every field present; a
// case value starts with its `$case` (J10).
func writeRecord(b *bytes.Buffer, r *value.Record) {
	b.WriteByte(openObject)
	first := true
	member := func(name string) {
		if !first {
			b.WriteByte(comma)
		}
		first = false
		writeString(b, name)
		b.WriteByte(colonByte)
	}
	if c, ok := r.T.Base().(*types.CaseType); ok {
		member(keyCase)
		writeString(b, c.Name)
	}
	for i, f := range FieldsOf(r.T) {
		member(f.Name)
		if i < len(r.Fields) {
			write(b, r.Fields[i])
		} else {
			b.WriteString(valueNull)
		}
	}
	b.WriteByte(closeObject)
}

// Float is a float of bits 64 or 32 as a view-model number, in its canonical text.
func Float(f float64, bits int) vm.Number { return vm.Number{Text: types.FloatText(f, bits)} }
