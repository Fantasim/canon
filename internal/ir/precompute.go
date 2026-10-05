package ir

import (
	"math"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// precompute evaluates every stored export fn of the assembled packages that have an emit other than view (EVALUATION.md §2.3): package fns once or per cell, methods per receiver reachable from the package's public values, then for a ts data emit from its field defaults (DECISIONS 278), in traversal order (§8.1), each receiver once. A stored result is encoded with its own `$` keys (WIRE.md §5.11, decision 128), so the receivers it holds are precomputed too, after the root that produced it.
func (s *stage) precompute() {
	pc := &precomputer{s: s, seen: map[*value.Record]bool{}}
	for _, u := range s.order {
		if !u.selected || !hasDataEmit(u) {
			continue
		}
		s.precomputeFns(pc, u)
		for _, v := range u.values {
			pc.walk(v.v.V)
		}
		pc.precomputeDefaults(u)
	}
}

// precomputeFns computes u's package fns; a `@text` result's stored fns are never written, so never evaluated (DECISIONS 308).
func (s *stage) precomputeFns(pc *precomputer, u *unit) {
	for _, site := range u.fns {
		if site.fn.Kind == FnTranslated || !s.computable(site) {
			continue
		}
		site.fn.Value, site.fn.Table, _ = s.results(site, nil)
		if !site.text {
			pc.walk(site.fn.Value)
			pc.walk(site.fn.Table)
		}
	}
}

// precomputer is one stage-E precomputation: the receivers already computed, and the stored results whose receivers wait their turn, each with the chain of declarations whose results led to it, so a chain of results never deepens the Go stack.
type precomputer struct {
	s       *stage
	seen    map[*value.Record]bool
	pending []pendingResult
	above   *declChain
}

// pendingResult is a stored result, or a lookup table, still to walk, and the receivers' declarations above it.
type pendingResult struct {
	v     any
	above *declChain
}

// declChain is the declarations (record or case) of the receivers whose stored results hold a value, innermost first.
type declChain struct {
	decl any
	up   *declChain
}

// holds reports d on the chain.
func (c *declChain) holds(d any) bool {
	for ; c != nil; c = c.up {
		if c.decl == d {
			return true
		}
	}
	return false
}

// walk computes the receivers of a root v in traversal order (EVALUATION.md §8.1), then those of the stored results met, first produced first.
func (pc *precomputer) walk(v any) {
	pc.queue(v, nil)
	for len(pc.pending) > 0 {
		next := pc.pending[0]
		pc.pending = pc.pending[1:]
		pc.above = next.above
		for _, x := range resultValues(next.v) {
			walkInstances(x, pc.seen, pc.receiver)
		}
	}
	pc.above = nil
}

// queue adds a stored result, or a lookup table, to the values still to walk.
func (pc *precomputer) queue(v any, above *declChain) {
	pc.pending = append(pc.pending, pendingResult{v: v, above: above})
}

// resultValues is a stored result alone, or each cell of a lookup table.
func resultValues(v any) []value.Value {
	switch x := v.(type) {
	case value.Value:
		return []value.Value{x}
	case *LookupTable:
		if x != nil {
			return x.Cells
		}
	}
	return nil
}

// receiver computes r's stored methods and queues their results. A receiver whose declaration already produced the result holding it is an internal error: cyclicFn refuses such a chain and its fns are never computed (DECISIONS 284).
func (pc *precomputer) receiver(r *value.Record) {
	decl := pc.s.declOf(r.T)
	if pc.above.holds(decl) {
		pc.s.endlessChain(decl)
		return
	}
	above := &declChain{decl: decl, up: pc.above}
	for _, in := range pc.s.receiver(r) {
		pc.queue(in.Result, above)
		pc.queue(in.Table, above)
	}
}

// precomputeDefaults walks the receivers of a ts data emit's field defaults, after the values (DECISIONS 278).
func (pc *precomputer) precomputeDefaults(u *unit) {
	if !tsData(u) {
		return
	}
	for _, d := range ownDefaults(u.p) {
		pc.walk(d)
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

// receiver computes the stored methods of one receiver: a record or case instance, of this package or an imported one, whose encoded `$` keys need them too (WIRE.md §5.11, decision 128), and returns the Instances it added. A cyclic method (DECISIONS 284) is not computed: E8019 refuses it. A failed call leaves out this receiver's Instance only: every other receiver is still evaluated, each failure reported by the host (decision 194).
func (s *stage) receiver(r *value.Record) []*Instance {
	var out []*Instance
	decl := s.declOf(r.T)
	_, methods := classBody(decl)
	for _, m := range methods {
		site := s.fnObjs[m]
		if m.Kind == FnTranslated || site == nil || !s.computable(site) || s.cyclicFn(m, decl) {
			continue
		}
		if result, table, ok := s.results(site, r); ok {
			in := &Instance{Recv: r, Result: result, Table: table}
			m.Instances = append(m.Instances, in)
			out = append(out, in)
		}
	}
	return out
}

// declOf is the record, or the case of a variant, a record value's type names; nil for any other type.
func (s *stage) declOf(t types.Type) any {
	switch d := t.Base().(type) {
	case *types.RecordType:
		if r, ok := s.named[d].(*Record); ok {
			return r
		}
	case *types.AppliedRecord:
		return s.declOf(d.Rec)
	case *types.CaseType:
		if v, ok := s.named[d.Variant].(*Variant); ok && d.Index >= 0 && d.Index < len(v.Cases) {
			return v.Cases[d.Index]
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

// lookupDomains sets each lookup's Domains, of every package the stage builds, so a reader of another package's record enumerates them with no receiver held (CODEGEN.md §5.10).
func (s *stage) lookupDomains() {
	for fn, site := range s.fnObjs { //canon:unordered each fn's own field is set, independently of the others
		if fn.Kind != FnLookup {
			continue
		}
		if domains, _, ok := s.domains(site); ok {
			fn.Domains = domains
		}
	}
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
