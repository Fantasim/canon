package ir

import (
	"math"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// precompute evaluates every stored export fn of the assembled packages that have an emit other than view (EVALUATION.md §2.3): package fns once or per cell, methods per receiver reachable from the package's public values, then for a ts data emit from its field defaults (DECISIONS 278), in traversal order (§8.1), each receiver once.
func (s *stage) precompute() {
	seen := map[*value.Record]bool{}
	for _, u := range s.order {
		if !u.selected || !hasDataEmit(u) {
			continue
		}
		for _, site := range u.fns {
			if site.fn.Kind != FnTranslated && s.computable(site) {
				site.fn.Value, site.fn.Table, _ = s.results(site, nil)
			}
		}
		for _, v := range u.values {
			walkInstances(v.v.V, seen, s.receiver)
		}
		s.precomputeDefaults(u, seen)
	}
}

// precomputeDefaults walks the receivers of a ts data emit's field defaults, after the values (DECISIONS 278).
func (s *stage) precomputeDefaults(u *unit, seen map[*value.Record]bool) {
	if !tsData(u) {
		return
	}
	for _, d := range ownDefaults(u.p) {
		walkInstances(d, seen, s.receiver)
	}
}

// tsData reports a ts emit in data mode: its readers write a constant field default as a literal, its records' stored results included (DECISIONS 278).
func tsData(u *unit) bool {
	for _, es := range u.emits {
		if es.e.Target == TargetTS && es.e.Mode == ModeData {
			return true
		}
	}
	return false
}

// ownDefaults are the constant defaults of the package's record and case fields, in declaration order.
func ownDefaults(p *Package) []value.Value {
	var out []value.Value
	for _, class := range tsClasses(p) {
		fields, _ := classBody(class)
		for _, f := range fields {
			if f.Default != nil && !f.Computed {
				out = append(out, f.Default)
			}
		}
	}
	return out
}

// hasDataEmit reports an emit other than view.
func hasDataEmit(u *unit) bool {
	for _, es := range u.emits {
		if es.e.Target != TargetView {
			return true
		}
	}
	return false
}

// receiver computes the stored methods of one receiver: a record or case instance, of this package or an imported one, whose encoded `$` keys need them too (WIRE.md §5.11, decision 128). A failed call leaves out this receiver's Instance only: every other receiver is still evaluated, each failure reported by the host (decision 194).
func (s *stage) receiver(r *value.Record) {
	for _, m := range s.methodsOf(r.T) {
		site := s.fnObjs[m]
		if m.Kind == FnTranslated || site == nil || !s.computable(site) {
			continue
		}
		if result, table, ok := s.results(site, r); ok {
			m.Instances = append(m.Instances, &Instance{Recv: r, Result: result, Table: table})
		}
	}
}

// methodsOf is the export methods of the declaration a record value's type names.
func (s *stage) methodsOf(t types.Type) []*ExportFn {
	switch d := t.Base().(type) {
	case *types.RecordType:
		if r, ok := s.named[d].(*Record); ok {
			return r.Methods
		}
	case *types.AppliedRecord:
		return s.methodsOf(d.Rec)
	case *types.CaseType:
		if v, ok := s.named[d.Variant].(*Variant); ok && d.Index < len(v.Cases) {
			return v.Cases[d.Index].Methods
		}
	}
	return nil
}

// computable reports a stored fn whose signature stage E accepts: no optional parameter
// (E9003) and at most maxCells cells (E9002).
func (s *stage) computable(site *fnSite) bool {
	if site.fn.Kind == FnPrecomputed {
		return true
	}
	_, n, ok := s.domains(site)
	return ok && n <= maxCells
}

// results is a fn's value for recv (nil for a package fn): a precomputed result, or the dense table over its domains, the first parameter varying slowest (CODEGEN.md §5.10); every cell is still called after one fails, so every failure is reported (EVALUATION.md §2.3, decision 194), and a failure is false with no result.
func (s *stage) results(site *fnSite, recv *value.Record) (value.Value, *LookupTable, bool) {
	var self value.Value
	if recv != nil {
		self = recv
	}
	if site.fn.Kind == FnPrecomputed {
		v, ok := s.in.Host.Call(s.ctx, site.obj, self, nil)
		if !ok {
			return nil, nil, false
		}
		return v, nil, true
	}
	domains, n, _ := s.domains(site)
	table := &LookupTable{Domains: domains, Cells: make([]value.Value, n)}
	args := make([]value.Value, len(domains))
	failed := false
	for c := range int(n) {
		rest := c
		for i := len(domains) - 1; i >= 0; i-- {
			args[i] = domains[i][rest%len(domains[i])]
			rest /= len(domains[i])
		}
		v, ok := s.in.Host.Call(s.ctx, site.obj, self, append([]value.Value(nil), args...))
		if !ok {
			failed = true
			continue
		}
		table.Cells[c] = v
	}
	if failed {
		return nil, nil, false
	}
	return nil, table, true
}

// domains are the values of each finite parameter in domain order (CODEGEN.md §5.10): enum members and table entries retired included, false then true; n is the number of cells.
func (s *stage) domains(site *fnSite) ([][]value.Value, int64, bool) {
	if d, ok := s.domainsOf[site.fn]; ok {
		return d.values, d.cells, d.ok
	}
	d := &fnDomains{values: make([][]value.Value, len(site.sig.Params)), cells: 1, ok: true}
	s.domainsOf[site.fn] = d
	for i, p := range site.sig.Params {
		values, ok := s.domain(p)
		if !ok {
			d.ok = false
			break
		}
		d.values[i] = values
		d.cells = cellProduct(d.cells, int64(len(values)))
	}
	return d.values, d.cells, d.ok
}

// fnDomains are the domains of one fn, computed once.
type fnDomains struct {
	values [][]value.Value
	cells  int64
	ok     bool
}

// cellProduct multiplies two counts, saturating at the largest int64.
func cellProduct(a, b int64) int64 {
	if b != 0 && a > math.MaxInt64/b {
		return math.MaxInt64
	}
	return a * b
}

func (s *stage) domain(t types.Type) ([]value.Value, bool) {
	switch x := t.Base().(type) {
	case *types.EnumType:
		out := make([]value.Value, len(x.Members))
		for i := range x.Members {
			out[i] = &value.Member{Enum: x, Index: i}
		}
		return out, true
	case *types.RefType:
		return s.refDomain(t, x)
	}
	if t.Base().Kind() == types.Bool {
		return []value.Value{&value.Bool{V: false}, &value.Bool{V: true}}, true
	}
	return nil, false
}

// refDomain is a ref into a table let: one ref per entry, in entry order.
func (s *stage) refDomain(t types.Type, r *types.RefType) ([]value.Value, bool) {
	c := r.Target
	if c == nil || c.KeyedBy != nil || c.Kind == types.CollField {
		return nil, false
	}
	v, ok := s.in.Host.Value(s.ctx, c.Pkg, c.Name)
	tbl, isTable := v.(*value.Table)
	if !ok || !isTable {
		return nil, false
	}
	out := make([]value.Value, len(tbl.Entries))
	for i, e := range tbl.Entries {
		out[i] = &value.Ref{T: t, Key: e.Ident.Key}
	}
	return out, true
}

// walkInstances visits every record and case instance of v depth first, pre-order: fields in declaration order, elements in order, map keys then values, table entries; refs are not followed and an instance met again is skipped (EVALUATION.md §8.1).
func walkInstances(v value.Value, seen map[*value.Record]bool, visit func(*value.Record)) {
	switch x := v.(type) {
	case *value.Record:
		walkRecord(x, seen, visit)
	case *value.List:
		walkEach(x.Elems, seen, visit)
	case *value.Map:
		walkMap(x, seen, visit)
	case *value.Table:
		for _, e := range x.Entries {
			walkRecord(e, seen, visit)
		}
	}
}

// walkRecord visits a record instance once, then its fields (EVALUATION.md §8.1).
func walkRecord(x *value.Record, seen map[*value.Record]bool, visit func(*value.Record)) {
	if seen[x] {
		return
	}
	seen[x] = true
	visit(x)
	walkEach(x.Fields, seen, visit)
}

// walkMap visits a map's keys then its values, in entry order.
func walkMap(x *value.Map, seen map[*value.Record]bool, visit func(*value.Record)) {
	for i := range x.Keys {
		walkInstances(x.Keys[i], seen, visit)
		walkInstances(x.Vals[i], seen, visit)
	}
}

// walkEach visits every value of vs in order.
func walkEach(vs []value.Value, seen map[*value.Record]bool, visit func(*value.Record)) {
	for _, v := range vs {
		walkInstances(v, seen, visit)
	}
}
