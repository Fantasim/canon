package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// checkSafeInts is E8101: every integer a TypeScript emit writes fits a number, unless its field has @ts(bigint) (CODEGEN.md §4.1, DECISIONS 279(d)): values with the stored results of the records they hold, constants, ref keys, and the package's precomputed fn results and lookup tables, whose finding (variant `result`) names the fn, which cannot be bigint. A fn or value the mode already refuses gets none, one finding per cause: no value's own data in `types` mode, which emits none (decision 194; selectedValues), nothing in a refused mode (decision 213), no package fn in data mode (E8013) or types mode (E8014), no method table keyed by a ref in data mode (E8013).
func (s *stage) checkSafeInts(u *unit, es *emitSite) {
	c := &safeInts{s: s, u: u, mode: es.e.Mode, byRecv: map[*ExportFn]map[*value.Record]*Instance{}, seen: map[*value.Record]bool{}}
	if !modeRefused(es.e) {
		for _, v := range selectedValues(u, es.e) {
			c.check(v.v.V, intSite{field: v.v.Name}, v.span().span())
		}
	}
	for _, cs := range u.consts {
		c.check(cs.c.V, intSite{field: cs.c.Name}, cs.span())
	}
	if modeRefused(es.e) || es.e.Mode == ModeData || es.e.Mode == ModeTypes {
		return // a package fn is E8013's in data mode and E8014's in types mode: one finding per cause
	}
	for _, site := range u.fns {
		if site.fn.Kind != FnTranslated {
			c.span = site.span()
			c.results(site.fn, site.fn.Value, site.fn.Table)
		}
	}
}

// safeInts finds the unsafe integers of one ts emit, reporting them at span, the value, constant or fn being checked; byRecv indexes each method's stored results by receiver; seen holds the records already checked, once each, since one record may be reached from several values or results.
type safeInts struct {
	s      *stage
	u      *unit
	mode   Mode
	byRecv map[*ExportFn]map[*value.Record]*Instance
	seen   map[*value.Record]bool
	span   source.Span
}

// intSite is what holds an integer: a field (with its @ts(bigint)), or, at a fn result's own positions, the fn.
type intSite struct {
	field, fn string
	big       bool
}

// check reports at span each unsafe integer of v held at site.
func (c *safeInts) check(v value.Value, site intSite, span source.Span) {
	c.span = span
	c.walk(v, site)
}

// results reports the unsafe integers of a stored fn result and of its lookup cells.
func (c *safeInts) results(fn *ExportFn, result value.Value, table *LookupTable) {
	site := intSite{fn: c.fnLabel(fn)}
	if result != nil {
		c.walk(result, site)
	}
	c.each(cellsOf(table), site)
}

// walk reports each integer of v outside ±(2^53 - 1), a ref's integer key included unless its key field is @ts(bigint) (DECISIONS 278); an element or map entry belongs to the site holding its collection, a record's field to the field.
func (c *safeInts) walk(v value.Value, site intSite) {
	switch x := v.(type) {
	case *value.Int:
		if !site.big && (x.V > maxSafeInt || x.V < -maxSafeInt) {
			c.report(x, site)
		}
	case *value.Ref:
		if key, ok := unsafeRefKey(x); ok {
			c.report(&value.Int{V: x.Key.I}, intSite{field: key})
		}
	case *value.Record:
		c.record(x)
	case *value.List:
		c.each(x.Elems, site)
	case *value.Map:
		c.each(x.Keys, site)
		c.each(x.Vals, site)
	case *value.Table:
		for _, e := range x.Entries {
			c.record(e)
		}
	}
}

func (c *safeInts) each(vs []value.Value, site intSite) {
	for _, v := range vs {
		c.walk(v, site)
	}
}

// record checks each field under its own name and @ts(bigint) flag, then the results its precomputed and finite-parameter fns store for it (CODEGEN.md §5.10, DECISIONS 279(d)).
func (c *safeInts) record(x *value.Record) {
	if c.seen[x] {
		return
	}
	c.seen[x] = true
	fields, fns := c.s.classOf(x.T)
	for i, fv := range x.Fields {
		if i < len(fields) {
			c.walk(fv, intSite{field: fields[i].Name, big: fields[i].BigInt})
		}
	}
	for _, fn := range fns {
		if in := c.instance(fn, x); in != nil && fn.Kind != FnTranslated && (c.mode != ModeData || !refKeyedLookup(fn)) {
			c.results(fn, in.Result, in.Table)
		}
	}
}

// instance is fn's stored result for the receiver recv, nil when none.
func (c *safeInts) instance(fn *ExportFn, recv *value.Record) *Instance {
	idx, ok := c.byRecv[fn]
	if !ok {
		idx = map[*value.Record]*Instance{}
		for _, in := range fn.Instances {
			idx[in.Recv] = in
		}
		c.byRecv[fn] = idx
	}
	return idx[recv]
}

// fnLabel is a fn as E8101 names it: `per` or `K.per`, qualified when another package declares it.
func (c *safeInts) fnLabel(fn *ExportFn) string {
	site := c.s.fnObjs[fn]
	if site == nil {
		return fn.Name
	}
	if site.pkg != c.u.p.Name {
		return site.pkg + qnameSep + site.label
	}
	return site.label
}

func (c *safeInts) report(n *value.Int, site intSite) {
	if site.fn != "" {
		c.u.report(diag.E8101.AtResult(c.span, n, site.fn))
		return
	}
	c.u.report(diag.E8101.AtField(c.span, n, site.field))
}

// classOf is the IR fields and export fns of the declaration a record value's type names.
func (s *stage) classOf(t types.Type) ([]*Field, []*ExportFn) {
	switch d := t.Base().(type) {
	case *types.RecordType:
		if r, ok := s.named[d].(*Record); ok {
			return r.Fields, r.Methods
		}
	case *types.AppliedRecord:
		return s.classOf(d.Rec)
	case *types.CaseType:
		if v, ok := s.named[d.Variant].(*Variant); ok && d.Index < len(v.Cases) {
			return classBody(v.Cases[d.Index])
		}
	}
	return nil, nil
}

// unsafeRefKey reports a ref whose integer key is outside ±(2^53 - 1) and whose key field, named for the fix, has no @ts(bigint) (DECISIONS 278).
func unsafeRefKey(x *value.Ref) (string, bool) {
	r, ok := x.T.Base().(*types.RefType)
	if !ok || r.Target == nil || r.Target.KeyedBy == nil || !x.Key.IsInt || bigKey(r.Target) {
		return "", false
	}
	return r.Target.KeyedBy.Name, x.Key.I > maxSafeInt || x.Key.I < -maxSafeInt
}

// refKeyedLookup reports a finite-parameter fn with a ref parameter: data files hold no table of one (E8013, CODEGEN.md §5.10).
func refKeyedLookup(fn *ExportFn) bool {
	return fn.Kind == FnLookup && slices.ContainsFunc(fn.Params, func(p *Param) bool { return p.Type.Kind == types.Ref })
}
