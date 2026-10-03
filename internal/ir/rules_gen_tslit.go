package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// checkTSLiterals is E8019 where a baked or embedded ts emit cannot write a literal (CODEGEN.md §5.6, §5.9, §5.10, DECISIONS 278): `DependentType` for a dependent value whose discriminant the literal does not read (in a map, a literal union, a fn result, or through a ref); `RecordCycleThroughMethod` for a value whose stored results hold their own receiver, a literal without end; `CrossPackageBakedValue` for a plain value of another package's record whose interface requires `id` (judge). Constants and stored fns are written too.
func (s *stage) checkTSLiterals(u *unit, es *emitSite) {
	lit := &tsLit{own: u.p.Name, cycles: newLiteralCycles(s), strict: s.tsStrictRows(u)}
	for _, v := range selectedValues(u, es.e) {
		if kind, bad := lit.judge(&v.v.Type, v.v.V); bad {
			u.reportGenConstruct(es, v.span().span(), kind)
		}
	}
	for _, c := range u.consts {
		if kind, bad := lit.judge(&c.c.Type, c.c.V); bad {
			u.reportGenConstruct(es, c.span(), kind)
		}
	}
	for _, site := range s.ownFns(u) {
		if site.fn.Kind == FnTranslated {
			continue
		}
		for _, r := range storedResults(site.fn) {
			if kind, bad := lit.judge(&site.fn.Result, r); bad {
				u.reportGenConstruct(es, site.span(), kind)
				break
			}
		}
	}
}

// tsLit judges literals gen/ts writes for package own.
type tsLit struct {
	own    string
	cycles *literalCycles
	strict func(*Record) bool
}

// judge is tsLiteral, then `CrossPackageBakedValue` for a plain value of another package's record whose interface requires `id` (a row there, never a plain value: CODEGEN.md §5.4).
func (l *tsLit) judge(t *TypeRef, v value.Value) (diag.Kind, bool) {
	if kind, bad := tsLiteral(l.own, t, v, l.cycles); bad {
		return kind, true
	}
	plainRow := literalHolds(t, v, func(t *TypeRef, v value.Value) bool {
		_, ok := v.(*value.Record)
		rec, isRec := t.Named.(*Record)
		return ok && isRec && t.Kind == types.Record && l.strict(rec)
	})
	return diag.KindCrossPackageBakedValue, plainRow
}

// tsStrictRows reports a record of another package that its own ts emit writes as a table row with a required `id`: one it never writes as a plain value (ir.TSRows there).
func (s *stage) tsStrictRows(u *unit) func(*Record) bool {
	cache := map[string]map[*Record]bool{}
	return func(r *Record) bool {
		if r.Pkg == u.p.Name || s.units[r.Pkg] == nil {
			return false
		}
		strict, ok := cache[r.Pkg]
		if !ok {
			strict = map[*Record]bool{}
			if e := tsEmitOf(u.p, r.Pkg); e != nil {
				rows, loose := TSRows(s.units[r.Pkg].p, e)
				for rec := range rows { //canon:unordered a set
					strict[rec] = !loose[rec]
				}
			}
			cache[r.Pkg] = strict
		}
		return strict[r]
	}
}

// tsLiteral is what gen/ts cannot write of v, of type t, as a literal: a dependent value it cannot place, or a literal without end.
func tsLiteral(own string, t *TypeRef, v value.Value, cycles *literalCycles) (diag.Kind, bool) {
	switch {
	case literalDependent(own, t, v, cppDiscRead):
		return diag.KindDependentType, true
	case cycles.endless(v):
		return diag.KindRecordCycleThroughMethod, true
	}
	return 0, false
}

// literalCycles finds values whose literal never ends: gen/ts writes a record value with its stored results, so a result that holds the receiver being written repeats it forever.
type literalCycles struct {
	byRecv map[*value.Record][]*Instance
	state  map[*value.Record]int
}

// newLiteralCycles indexes every stored result of every package by its receiver.
func newLiteralCycles(s *stage) *literalCycles {
	c := &literalCycles{byRecv: map[*value.Record][]*Instance{}, state: map[*value.Record]int{}}
	for _, u := range s.order {
		for _, class := range tsClasses(u.p) {
			_, fns := classBody(class)
			c.index(fns)
		}
	}
	return c
}

// index adds the stored results of fns by receiver.
func (c *literalCycles) index(fns []*ExportFn) {
	for _, fn := range fns {
		for _, in := range fn.Instances {
			c.byRecv[in.Recv] = append(c.byRecv[in.Recv], in)
		}
	}
}

// endless reports a record value of v met again while its own literal, stored results included, is being written.
func (c *literalCycles) endless(v value.Value) bool {
	switch x := v.(type) {
	case *value.Record:
		return c.record(x)
	case *value.List:
		return slices.ContainsFunc(x.Elems, c.endless)
	case *value.Map:
		return slices.ContainsFunc(x.Keys, c.endless) || slices.ContainsFunc(x.Vals, c.endless)
	case *value.Table:
		return slices.ContainsFunc(x.Entries, c.record)
	}
	return false
}

func (c *literalCycles) record(r *value.Record) bool {
	switch c.state[r] {
	case cycleOpen:
		return true
	case cycleDone:
		return false
	}
	c.state[r] = cycleOpen
	found := slices.ContainsFunc(r.Fields, c.endless) || slices.ContainsFunc(c.byRecv[r], func(in *Instance) bool {
		return c.endless(in.Result) || in.Table != nil && slices.ContainsFunc(in.Table.Cells, c.endless)
	})
	c.state[r] = cycleDone
	return found
}

// tsDefault is what a ts reader cannot write of a field's constant default (DECISIONS 278): a dependent field's present default, whose branch an absent key's discriminant decides at run time (as gen/cpp, checkCppDefaults), or a default whose stored results hold their own receiver.
func tsDefault(f *Field, cycles *literalCycles) (diag.Kind, bool) {
	switch {
	case f.Computed || !written(f.Default):
		return 0, false
	case typeHolds(&f.Type, isApp):
		return diag.KindDependentType, true
	case cycles.endless(f.Default):
		return diag.KindRecordCycleThroughMethod, true
	}
	return 0, false
}

// checkTSForeignTables is E8019 in every ts mode (CODEGEN.md §5.3, §5.4): a table of another package's record, held anywhere, has rows with `id` and `retired` that the record's interface in its own module does not declare, and no reader here reads them; `CrossPackageBakedValue` in baked and embedded mode, `ForeignDataRecord` in data and types mode, where readers read it.
func (s *stage) checkTSForeignTables(u *unit, es *emitSite) {
	own, kind := u.p.Name, diag.KindForeignDataRecord
	if !decodes(es.e) {
		kind = diag.KindCrossPackageBakedValue
	}
	for _, site := range s.typeSites(u, es) {
		if typeHolds(site.t, func(t *TypeRef) bool { return foreignTable(own, *t) }) {
			u.reportGenConstruct(es, site.span, kind)
		}
	}
}

// decodes reports a ts emit whose module has readers: data and types mode.
func decodes(e *Emit) bool { return e.Mode == ModeData || e.Mode == ModeTypes }
