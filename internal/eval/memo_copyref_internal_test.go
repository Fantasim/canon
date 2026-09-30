package eval

import (
	"reflect"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// refGraph is the reference: the map-based kept graph the slot tables replaced, copied apart once
// with a stand-in for each node read, then again on each replay through a map.
type refGraph struct {
	root    value.Value
	nodes   []value.Value
	marks   refMarks
	foreign []refForeign
}

type refForeign struct {
	stand     value.Value
	read, pos int
}

type refMarks struct {
	invalid, written         []value.Value
	history, rebuilt, origin []frozenLink
	bound                    []refBound
}

type refBound struct {
	rec    value.Value
	params map[*types.Param]value.Value
}

// refKeep is what the map-based freezer kept of the walk f made from root.
func refKeep(f *freezer, root value.Value) refGraph {
	done := map[value.Value]value.Value{}
	var out refGraph
	for n, fa := range f.foreign { //canon:unordered filling a map
		stand := reflect.New(fa.typ.Elem()).Interface().(value.Value)
		done[n] = stand
		out.foreign = append(out.foreign, refForeign{stand: stand, read: fa.read, pos: fa.pos})
	}
	refCopyNodes(f.nodes, done)
	out.root = refGet(done, root)
	m := &f.marks
	out.marks = refMarks{
		invalid: refGetAll(done, m.invalid), written: refGetAll(done, m.written),
		history: refLinks(done, m.history), rebuilt: refLinks(done, m.rebuilt), origin: refLinks(done, m.origin),
	}
	for _, rec := range m.bound {
		out.marks.bound = append(out.marks.bound, refBound{rec: refGet(done, rec), params: refParams(done, f.e.bound[rec])})
	}
	for _, n := range f.nodes {
		if nv, ok := done[n]; ok && refShell(n) != nil {
			out.nodes = append(out.nodes, nv)
		}
	}
	return out
}

// seed maps each stand-in to the node read now at its place.
func (g refGraph) seed(infos []*readInfo) (map[value.Value]value.Value, bool) {
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

// refThaw is the map-based copy of g for e, its marks set.
func (e *Evaluator) refThaw(g refGraph, done map[value.Value]value.Value) value.Value {
	refCopyNodes(g.nodes, done)
	m := g.marks
	for _, v := range m.invalid {
		e.invalid[refGet(done, v)] = true
	}
	for _, v := range m.written {
		e.written[refGet(done, v)] = true
		e.gens.written++
	}
	for _, l := range m.history {
		e.history[refGet(done, l.from)] = refGet(done, l.to)
	}
	for _, l := range m.rebuilt {
		e.rebuilt[refGet(done, l.from)] = refGet(done, l.to)
	}
	for _, l := range m.origin {
		e.origin[refGet(done, l.from).(*value.Record)] = refGet(done, l.to).(*value.Record)
	}
	for _, b := range m.bound {
		e.bound[refGet(done, b.rec).(*value.Record)] = refParams(done, b.params)
	}
	return refGet(done, g.root)
}

func refCopyNodes(nodes []value.Value, done map[value.Value]value.Value) {
	for _, n := range nodes {
		if nv := refShell(n); nv != nil {
			done[n] = nv
		}
	}
	for _, n := range nodes {
		if _, copied := done[n]; copied {
			refFill(done, n)
		}
	}
}

func refShell(n value.Value) value.Value {
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

func refFill(done map[value.Value]value.Value, n value.Value) {
	switch x := n.(type) {
	case *value.Record:
		cp := done[n].(*value.Record)
		cp.Fields = refGetAll(done, x.Fields)
		if x.Ident != nil {
			id := *x.Ident
			id.Owner = refGetRecord(done, x.Ident.Owner)
			cp.Ident = &id
		}
	case *value.List:
		done[n].(*value.List).Elems = refGetAll(done, x.Elems)
	case *value.Map:
		cp := done[n].(*value.Map)
		cp.Keys, cp.Vals = refGetAll(done, x.Keys), refGetAll(done, x.Vals)
	case *value.Table:
		cp := done[n].(*value.Table)
		cp.Entries = make([]*value.Record, len(x.Entries))
		for i, en := range x.Entries {
			cp.Entries[i] = refGetRecord(done, en)
		}
	case *value.Pair:
		cp := done[n].(*value.Pair)
		cp.A, cp.B = refGet(done, x.A), refGet(done, x.B)
	case *value.Ref:
		done[n].(*value.Ref).Owner = refGetRecord(done, x.Owner)
	}
}

func refGet(done map[value.Value]value.Value, v value.Value) value.Value {
	if nv, ok := done[v]; ok {
		return nv
	}
	return v
}

func refGetAll(done map[value.Value]value.Value, vs []value.Value) []value.Value {
	out := make([]value.Value, len(vs))
	for i, v := range vs {
		if v != nil {
			out[i] = refGet(done, v)
		}
	}
	return out
}

func refGetRecord(done map[value.Value]value.Value, rec *value.Record) *value.Record {
	if rec == nil {
		return nil
	}
	return refGet(done, rec).(*value.Record)
}

func refParams(done map[value.Value]value.Value, params map[*types.Param]value.Value) map[*types.Param]value.Value {
	out := make(map[*types.Param]value.Value, len(params))
	for p, arg := range params { //canon:unordered copied into a map
		out[p] = refGet(done, arg)
	}
	return out
}

func refLinks(done map[value.Value]value.Value, links []frozenLink) []frozenLink {
	out := make([]frozenLink, len(links))
	for i, l := range links {
		out[i] = frozenLink{from: refGet(done, l.from), to: refGet(done, l.to)}
	}
	return out
}
