package gogen

import (
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// valueInfo is an emitted value: its member of the baked data and its entries' positions.
type valueInfo struct {
	v     *ir.Value
	store string
	index map[value.Key]int
}

// indexValues finds the emitted values and the records that get a table's id type (§5.3).
func (g *gen) indexValues() {
	for _, v := range g.p.Values {
		if v.Type.Kind != types.Table {
			continue
		}
		rec, ok := g.sub(v.Type.Elem).Named.(*ir.Record)
		switch {
		case !ok || rec.Pkg != g.p.Name:
			g.failf(ErrUnsupported, "table %s of a record of another package", v.Name)
		case g.tableOf[rec] != nil:
			g.failf(ErrNameCollision, "%s is the id type of both %s and %s", idTypeName(g.goName(rec)), g.tableOf[rec].Name, v.Name)
		default:
			g.tableOf[rec] = v
		}
	}
	for _, v := range g.p.Values {
		if len(g.e.Values) > 0 && !slices.Contains(g.e.Values, v.Name) {
			continue
		}
		g.emitted = append(g.emitted, v)
		g.byValue[v.Name] = &valueInfo{v: v, store: storageName(v.Name), index: g.entryIndex(v)}
	}
}

// entryIndex maps each entry's key to its position: tables by id, keyed lists by key. A
// table's id enum numbers v.IDs, and its rows are indexed by it: both orders must agree.
func (g *gen) entryIndex(v *ir.Value) map[value.Key]int {
	index := map[value.Key]int{}
	isTable := v.Type.Kind == types.Table
	for i, r := range g.entries(v) {
		switch {
		case r.Ident == nil:
			g.failf(ErrMalformed, "entry %d of %s has no key", i, v.Name)
			continue
		case isTable && (i >= len(v.IDs) || v.IDs[i] != r.Ident.Key.S):
			g.failf(ErrMalformed, "entry %d of table %s is %s, not its id %d", i, v.Name, r.Ident.Key.Text(), i)
		}
		index[r.Ident.Key] = i
	}
	if isTable && len(index) != len(v.IDs) {
		g.failf(ErrMalformed, "table %s has %d ids for %d entries", v.Name, len(v.IDs), len(index))
	}
	return index
}

// entries is the rows of a table or keyed-list value, nil for any other value.
func (g *gen) entries(v *ir.Value) []*value.Record {
	switch {
	case v.Type.Kind == types.Table:
		return as[value.Table](g, v.V).Entries
	case v.Type.Kind == types.List && v.Type.KeyedBy != nil:
		elems := as[value.List](g, v.V).Elems
		out := make([]*value.Record, len(elems))
		for i, x := range elems {
			out[i] = as[value.Record](g, x)
		}
		return out
	}
	return nil
}

// indexInstances maps each record method's receivers to their precomputed results.
func (g *gen) indexInstances() {
	g.instances = map[*ir.ExportFn]map[*value.Record]*ir.Instance{}
	for _, t := range g.p.Types {
		switch t := t.(type) {
		case *ir.Record:
			g.addInstances(t.Methods)
		case *ir.Variant:
			for _, c := range t.Cases {
				g.addInstances(c.Methods)
			}
		}
	}
}

func (g *gen) addInstances(fns []*ir.ExportFn) {
	for _, fn := range fns {
		byRecv := map[*value.Record]*ir.Instance{}
		for _, in := range fn.Instances {
			byRecv[in.Recv] = in
		}
		g.instances[fn] = byRecv
	}
}

func (g *gen) instanceOf(fn *ir.ExportFn, origin string, r *value.Record) *ir.Instance {
	if in := g.instances[fn][r]; in != nil {
		return in
	}
	g.failf(ErrMalformed, "no precomputed result of %s for a receiver", origin)
	return &ir.Instance{}
}

// resolvable reports a ref into a value of this package and emit: its getter returns the entry (§5.8).
func (g *gen) resolvable(r *ir.RefTarget) bool {
	return r.Coll == types.CollLet && !r.Local && r.Pkg == g.p.Name && g.byValue[r.Value] != nil
}

// resolvedExpr points at the entry of an emitted value whose key is k, in the baked data d.
func (g *gen) resolvedExpr(r *ir.RefTarget, k value.Key) string {
	info := g.byValue[r.Value]
	i, ok := info.index[k]
	if !ok {
		g.failf(ErrMalformed, "%s has no entry %s", r.Value, k.Text())
	}
	g.usedData = true
	return g.data + dot + info.store + atCall + strconv.Itoa(i) + rparen
}

// entryPointer is the entry a record value is, when it belongs to an emitted table here.
func (g *gen) entryPointer(r *value.Record) (string, bool) {
	id := r.Ident
	if id == nil || id.Coll == nil || id.Coll.Kind != types.CollLet || id.Coll.Pkg != g.p.Name {
		return "", false
	}
	if g.byValue[id.Coll.Name] == nil {
		return "", false
	}
	target := &ir.RefTarget{Coll: types.CollLet, Pkg: id.Coll.Pkg, Value: id.Coll.Name}
	return g.resolvedExpr(target, id.Key), true
}
