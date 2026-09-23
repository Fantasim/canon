package wire

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Fn is a stored export fn: its Result, or Cells over Domains, the first varying slowest.
type Fn struct {
	Name    string
	Result  value.Value
	Domains [][]value.Value
	Cells   []value.Value
}

// Methods lists the `$` keys of a record or case value, in declaration order; nil for none.
type Methods func(r *value.Record) []Fn

// Document is the data file of one value; Kind is its declared type's kind (WIRE.md §8.2).
type Document struct {
	Schema  string
	Kind    types.Kind
	V       value.Value
	Fns     []Fn
	Methods Methods
}

// Encode is the file's canonical bytes (WIRE.md §7, §8.2).
func (d *Document) Encode() ([]byte, error) {
	if !schemaPattern.MatchString(d.Schema) {
		return nil, fmt.Errorf("%w: %q", ErrSchema, d.Schema)
	}
	e := &encoder{methods: d.Methods}
	b := append([]byte(openObject+newline+indentSpaces), diag.AppendJSONString(nil, keySchema)...)
	b = diag.AppendJSONString(append(b, sepKey...), d.Schema)
	b, err := e.main(b, d.Kind, d.V)
	if err == nil && len(d.Fns) > 0 {
		var fns *node
		if fns, err = e.fns(d.Fns); err == nil {
			b = fns.pretty(memberHead(b, keyFns), memberDepth)
		}
	}
	if err != nil {
		return nil, err
	}
	return append(b, newline+closeObject+newline...), nil
}

// main writes `rows` for a list, keyed list or table, else `value` (§8.2).
func (e *encoder) main(b []byte, kind types.Kind, v value.Value) ([]byte, error) {
	if kind == types.List || kind == types.Table {
		return e.rows(b, v)
	}
	n, err := e.value(v, scope{})
	if err != nil {
		return nil, err
	}
	return n.pretty(memberHead(b, keyValue), memberDepth), nil
}

// rows writes a list, keyed list or table as one compact row per line (§8.2).
func (e *encoder) rows(b []byte, v value.Value) ([]byte, error) {
	rows, err := e.rowNodes(v)
	if err != nil {
		return nil, err
	}
	b = memberHead(b, keyRows)
	if len(rows.elems) == 0 {
		return append(b, openArray+closeArray...), nil
	}
	b = append(b, openArray...)
	inner := newline + strings.Repeat(indentSpaces, rowDepth)
	for i, r := range rows.elems {
		b = r.compact(append(b, separator(i, inner)...))
	}
	return append(b, newline+indentSpaces+closeArray...), nil
}

// rowNodes encodes the elements, or the table entries with `$id` and `$retired` first (§5.7).
func (e *encoder) rowNodes(v value.Value) (*node, error) {
	switch x := v.(type) {
	case *value.List:
		return e.list(x.Elems, scope{})
	case *value.Table:
		rows := arrayNode(len(x.Entries))
		for _, r := range x.Entries {
			if r.Ident == nil {
				return nil, fmt.Errorf("%w: table row without identity", ErrShape)
			}
			head := objectNode()
			head.add(keyID, stringNode(r.Ident.Key.Text()))
			if r.Ident.Retired {
				head.add(keyRetired, text(textTrue))
			}
			row, err := e.record(r, head)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", r.Ident.Key.Text(), err)
			}
			rows.elems = append(rows.elems, row)
		}
		return rows, nil
	}
	return nil, fmt.Errorf("%w: rows of %T", ErrShape, v)
}

// fns is the `$fns` object: each package fn under its Canon name (§5.11).
func (e *encoder) fns(fns []Fn) (*node, error) {
	obj := objectNode()
	for _, fn := range fns {
		n, err := e.fn(fn)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fn.Name, err)
		}
		obj.add(fn.Name, n)
	}
	return obj, nil
}

func memberHead(b []byte, key string) []byte {
	return append(diag.AppendJSONString(append(b, sepPretty+indentSpaces...), key), sepKey...)
}
