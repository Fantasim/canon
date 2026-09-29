package eval

import (
	"reflect"
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// copyNodes copies into done each node that is not shared, then links the copies as the nodes
// are, through done's copies and stand-ins.
func copyNodes(nodes []value.Value, done map[value.Value]value.Value) {
	for _, n := range nodes {
		if nv := shell(n); nv != nil {
			done[n] = nv
		}
	}
	for _, n := range nodes {
		if _, copied := done[n]; copied {
			fill(done, n)
		}
	}
}

// shell is a new node like n, its parts left to fill; nil for a node shared as it is.
func shell(n value.Value) value.Value {
	switch x := n.(type) {
	case *value.Record:
		return &value.Record{T: x.T, Set: slices.Clone(x.Set), P: x.P}
	case *value.List:
		return &value.List{T: x.T, P: x.P}
	case *value.Map:
		return &value.Map{T: x.T, P: x.P}
	case *value.Table:
		return &value.Table{T: x.T, P: x.P}
	case *value.Pair:
		return &value.Pair{T: x.T, P: x.P}
	case *value.Ref:
		if x.Owner != nil {
			return &value.Ref{T: x.T, Key: x.Key, P: x.P}
		}
	}
	return nil
}

// fill links n's copy to the copies of n's parts.
func fill(done map[value.Value]value.Value, n value.Value) {
	switch x := n.(type) {
	case *value.Record:
		cp := done[n].(*value.Record)
		cp.Fields = getAll(done, x.Fields)
		if x.Ident != nil {
			id := *x.Ident
			id.Owner = getRecord(done, x.Ident.Owner)
			cp.Ident = &id
		}
	case *value.List:
		done[n].(*value.List).Elems = getAll(done, x.Elems)
	case *value.Map:
		cp := done[n].(*value.Map)
		cp.Keys, cp.Vals = getAll(done, x.Keys), getAll(done, x.Vals)
	case *value.Table:
		cp := done[n].(*value.Table)
		cp.Entries = make([]*value.Record, len(x.Entries))
		for i, en := range x.Entries {
			cp.Entries[i] = getRecord(done, en)
		}
	case *value.Pair:
		cp := done[n].(*value.Pair)
		cp.A, cp.B = get(done, x.A), get(done, x.B)
	case *value.Ref:
		done[n].(*value.Ref).Owner = getRecord(done, x.Owner)
	}
}

// get is v's copy or the node it stands for, v itself when shared.
func get(done map[value.Value]value.Value, v value.Value) value.Value {
	if nv, ok := done[v]; ok {
		return nv
	}
	return v
}

func getAll(done map[value.Value]value.Value, vs []value.Value) []value.Value {
	out := make([]value.Value, len(vs))
	for i, v := range vs {
		if v != nil {
			out[i] = get(done, v)
		}
	}
	return out
}

func getRecord(done map[value.Value]value.Value, rec *value.Record) *value.Record {
	if rec == nil {
		return nil
	}
	return get(done, rec).(*value.Record)
}

// standIn is an empty node of n's kind, which a kept graph holds in n's place.
func standIn(n value.Value) value.Value {
	return reflect.New(reflect.TypeOf(n).Elem()).Interface().(value.Value)
}

// seed maps each stand-in of g to the node of the values read now, infos, at its place; false
// when one is of another kind, which equal fingerprints exclude but a replay checks.
func (g memoGraph) seed(infos []*readInfo) (map[value.Value]value.Value, bool) {
	done := make(map[value.Value]value.Value, len(g.nodes)+len(g.foreign))
	for _, fa := range g.foreign {
		nodes := infos[fa.read].nodes
		if fa.pos >= len(nodes) || reflect.TypeOf(nodes[fa.pos]) != reflect.TypeOf(fa.stand) {
			return nil, false
		}
		done[fa.stand] = nodes[fa.pos]
	}
	return done, true
}

// thaw is a new copy of g for e, linked to the nodes done seeds, its marks set in e as the
// entry's evaluation set them.
func (e *Evaluator) thaw(g memoGraph, done map[value.Value]value.Value) *value.Record {
	copyNodes(g.nodes, done)
	m := g.marks
	for _, v := range m.invalid {
		e.invalid[get(done, v)] = true
	}
	for _, v := range m.written {
		e.written[get(done, v)] = true
		e.gens.written++
	}
	for _, l := range m.history {
		e.history[get(done, l.from)] = get(done, l.to)
	}
	for _, l := range m.rebuilt {
		e.rebuilt[get(done, l.from)] = get(done, l.to)
	}
	for _, l := range m.origin {
		e.origin[get(done, l.from).(*value.Record)] = get(done, l.to).(*value.Record)
	}
	for _, b := range m.bound {
		e.bound[get(done, b.rec).(*value.Record)] = movedParams(done, b.params)
	}
	return getRecord(done, g.root)
}
