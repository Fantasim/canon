package edit

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// wireNode is v in the source wire at field f's place, f nil for no field (API.md M8): records
// without the fields a literal leaves out (M7), symbols as their branch names them (DEP-02); an
// inline or pairs field gives the members it writes in its parent, as an object.
func (a *applier) wireNode(v value.Value, f *types.Field) (*jsonsrc.Node, error) {
	sv, err := depFrame{a: a}.symbolsIn(v, nil, nil)
	if err != nil {
		return nil, err
	}
	rv, err := a.restrict(sv)
	if err != nil {
		return nil, err
	}
	return encodeWire(rv, f)
}

// encodeWire is v, whose records hold only what they write, in the source wire at f's place.
func encodeWire(v value.Value, f *types.Field) (*jsonsrc.Node, error) {
	sf := &types.Field{Name: wireSlot, Wire: wireSlot, WirePath: []string{wireSlot}, Type: v.Type()}
	if f != nil {
		cp := *f
		cp.Index, cp.Name, cp.Wire, cp.WirePath, cp.Type = 0, wireSlot, wireSlot, []string{wireSlot}, v.Type()
		sf = &cp
	}
	holder := &value.Record{T: &types.RecordType{Name: wireSlot, Fields: []*types.Field{sf}}, Fields: []value.Value{v}, Set: []bool{true}}
	doc := wire.Document{Schema: wireSchema, Kind: types.Record, V: holder}
	out, err := doc.Encode()
	if err != nil {
		return nil, fmt.Errorf(fmtWrapped, errNoWire, err)
	}
	var fs source.FileSet
	src, err := fs.Add(wireSlot, wireSlot, out)
	if err != nil {
		return nil, err
	}
	root, err := jsonsrc.Parse(src, diag.NewBag(&fs, ""))
	if err != nil {
		return nil, err
	}
	at := wireValuePtr
	if sf.Inline || sf.Pairs != nil {
		return detached(root.Find(at)), nil
	}
	return detached(root.Find(at + pointerSep + wireSlot)), nil
}

// retiredMember is the member the encoder writes first in a retired table entry (WIRE.md 5.7).
func retiredMember() (jsonsrc.Member, error) {
	rt := &types.RecordType{Name: wireSlot}
	entry := &value.Record{T: rt, Ident: &value.Identity{Key: value.Key{S: wireSlot}, Retired: true}}
	n, err := encodeWire(&value.Table{T: &types.TableType{Elem: rt}, Entries: []*value.Record{entry}}, nil)
	if err != nil {
		return jsonsrc.Member{}, err
	}
	e := n.Find(pointerSep + wireSlot)
	if e == nil || len(e.Members) == 0 {
		return jsonsrc.Member{}, errNoWire
	}
	return e.Members[0], nil
}

// detached is a copy of n that no longer points into the document it was read from.
func detached(n *jsonsrc.Node) *jsonsrc.Node {
	if n == nil {
		return nil
	}
	out := &jsonsrc.Node{Kind: n.Kind, Text: n.Text}
	for _, e := range n.Elems {
		out.Elems = append(out.Elems, detached(e))
	}
	for _, m := range n.Members {
		out.Members = append(out.Members, jsonsrc.Member{Key: m.Key, Value: detached(m.Value)})
	}
	return out
}

// wireText is v's source wire at f's place, compact, as Dropped reports it (API.md §8.1).
func (a *applier) wireText(v value.Value, f *types.Field) (json.RawMessage, error) {
	n, err := a.wireNode(v, f)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Compact(&b, jsonsrc.Format(n)); err != nil {
		return nil, fmt.Errorf(fmtWrapped, errNoWire, err)
	}
	return b.Bytes(), nil
}

// restrict is a copy of v whose records keep only the fields a literal writes (M7), for the data
// wire's encoder; a symbol left is the string the decoder reads a symbol from (DECISIONS 175).
func (a *applier) restrict(v value.Value) (value.Value, error) {
	switch x := v.(type) {
	case *value.Record:
		return a.restrictRecord(x)
	case *value.List:
		elems, err := a.restrictAll(x.Elems)
		return &value.List{T: x.T, Elems: elems, P: x.P}, err
	case *value.Map:
		keys, err := a.restrictAll(x.Keys)
		if err != nil {
			return nil, err
		}
		vals, err := a.restrictAll(x.Vals)
		return &value.Map{T: x.T, Keys: keys, Vals: vals, P: x.P}, err
	case *value.Table:
		out := &value.Table{T: x.T, P: x.P}
		for _, e := range x.Entries {
			r, err := a.restrictRecord(e)
			if err != nil {
				return nil, err
			}
			out.Entries = append(out.Entries, r)
		}
		return out, nil
	case *value.Symbol:
		return &value.Str{V: x.Name, T: types.StringType, P: x.P}, nil
	}
	return v, nil
}

func (a *applier) restrictAll(vs []value.Value) ([]value.Value, error) {
	out := make([]value.Value, len(vs))
	for i, v := range vs {
		r, err := a.restrict(v)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

// restrictRecord is r with only its printed fields, under a copy of its type holding those.
func (a *applier) restrictRecord(r *value.Record) (*value.Record, error) {
	fields := fieldsOf(r.T)
	var keep []*types.Field
	var vals []value.Value
	for i, f := range fields {
		if !a.printed(r, i) {
			continue
		}
		v, err := a.restrict(r.Fields[i])
		if err != nil {
			return nil, err
		}
		keep, vals = append(keep, f), append(vals, v)
	}
	set := make([]bool, len(keep))
	for i := range set {
		set[i] = true
	}
	return &value.Record{T: restrictedType(r.T, keep), Fields: vals, Set: set, Ident: r.Ident, P: r.P}, nil
}

// restrictedType is a copy of a record or case type declaring only fields.
func restrictedType(t types.Type, fields []*types.Field) types.Type {
	switch b := t.Base().(type) {
	case *types.CaseType:
		c := *b
		c.Fields = fields
		return &c
	case *types.AppliedRecord:
		r := *b.Rec
		r.Fields = fields
		return &r
	case *types.RecordType:
		r := *b
		r.Fields = fields
		return &r
	}
	return t
}
