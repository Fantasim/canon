package ir

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
)

// cycleReport is one emit's E8019 for one fn: one finding per cause, however many packages reach the fn.
type cycleReport struct {
	fn     *ExportFn
	target Target
	mode   Mode
}

// cyclicFn reports a stored export method whose result type reaches its receiver's own declaration by value, through records, cases, lists, maps and optionals (DECISIONS 284): it has no finite set of results to precompute or write. owner is the receiver's record or case. The one judgement every target uses.
func (s *stage) cyclicFn(fn *ExportFn, owner any) bool {
	if fn.Kind == FnTranslated || owner == nil {
		return false
	}
	if c, ok := s.cyclic[fn]; ok {
		return c
	}
	seen := map[any]bool{}
	c := false
	for _, d := range heldClasses(&fn.Result, nil) {
		if c = classReaches(d, owner, seen); c {
			break
		}
	}
	s.cyclic[fn] = c
	return c
}

// classReaches reports that from is to or holds it by value, through its fields and its stored methods' results.
func classReaches(from, to any, seen map[any]bool) bool {
	if from == to {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	for _, d := range classEdges(from) {
		if classReaches(d, to, seen) {
			return true
		}
	}
	return false
}

// classEdges are the records and cases a record or case holds by value: its fields', then its stored methods' results.
func classEdges(class any) []any {
	fields, fns := classBody(class)
	var out []any
	for _, f := range fields {
		out = heldClasses(&f.Type, out)
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = heldClasses(&fn.Result, out)
		}
	}
	return out
}

// heldClasses adds the records and cases a value of t holds by value: through lists, maps (keys and values), optionals and dependent branches, never through a ref.
func heldClasses(t *TypeRef, out []any) []any {
	if t == nil {
		return out
	}
	switch n := t.Named.(type) {
	case *Record:
		out = append(out, n)
	case *Variant:
		out = appendCases(out, n, t.Case)
	case *Dependent:
		for _, b := range n.Branches {
			out = heldClasses(&b.Type, out)
		}
	}
	return heldClasses(t.Key, heldClasses(t.Elem, out))
}

// appendCases adds one case, or every case of v.
func appendCases(out []any, v *Variant, one *Case) []any {
	if one != nil {
		return append(out, one)
	}
	for _, c := range v.Cases {
		out = append(out, c)
	}
	return out
}

// reachedClasses are the records and cases u's emits can write, in a fixed order: its own, those its values and package fns hold, then what they hold in turn, other packages' included (decision 194).
func reachedClasses(u *unit) []any {
	var start []any
	for _, t := range u.p.Types {
		start = heldClasses(&TypeRef{Named: t}, start)
	}
	for _, v := range u.p.Values {
		start = heldClasses(&v.Type, start)
	}
	for _, fn := range u.p.Fns {
		if fn.Kind != FnTranslated {
			start = heldClasses(&fn.Result, start)
		}
	}
	seen := map[any]bool{}
	var out []any
	for len(start) > 0 {
		c := start[0]
		start = start[1:]
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
			start = append(start, classEdges(c)...)
		}
	}
	return out
}

// carriesResults reports an emit that writes stored results: json, and a code emit in any mode but types, where E8014 refuses them (DECISIONS 284).
func carriesResults(e *Emit) bool {
	return e.Target == TargetJSON || isCode(e.Target) && e.Mode != ModeTypes
}

// checkMethodCycles is E8019 `RecordCycleThroughMethod` (DECISIONS 284): every cyclic stored method an emit of u writes, at the fn, in its own package's findings; the json variant for `emit json`, which has no mode.
func (s *stage) checkMethodCycles(u *unit) {
	var classes []any
	for _, es := range u.emits {
		if es.index > 0 || !carriesResults(es.e) {
			continue
		}
		if classes == nil {
			classes = reachedClasses(u)
		}
		s.reportCycles(es.e, classes)
	}
}

// reportCycles reports, for e, each cyclic stored method of classes.
func (s *stage) reportCycles(e *Emit, classes []any) {
	for _, c := range classes {
		_, fns := classBody(c)
		for _, fn := range fns {
			if s.cyclicFn(fn, c) {
				s.reportCycle(e, fn)
			}
		}
	}
}

// reportCycle reports fn's cycle for e once.
func (s *stage) reportCycle(e *Emit, fn *ExportFn) {
	key := cycleReport{fn: fn, target: e.Target, mode: e.Mode}
	// Never nil: stage E builds every ExportFn with its fnSite (s.methods), and newStage a unit for every package of the program, site.pkg's included.
	site := s.fnObjs[fn]
	if s.cycleReported[key] {
		return
	}
	s.cycleReported[key] = true
	if e.Target == TargetJSON {
		s.units[site.pkg].report(diag.E8019.AtJson(site.span(), diag.KindRecordCycleThroughMethod, wayOf[diag.KindRecordCycleThroughMethod]))
		return
	}
	s.units[site.pkg].report(diag.E8019.AtMode(site.span(), targetWords[e.Target], modeWords[e.Mode], diag.KindRecordCycleThroughMethod, wayOf[diag.KindRecordCycleThroughMethod]))
}

// endlessChain is the internal error of a stored result that holds, below it, a receiver of a declaration whose results led to it: cyclicFn refuses every such chain first, so stage E is broken (DECISIONS 284).
func (s *stage) endlessChain(class any) {
	_, fns := classBody(class)
	for _, fn := range fns {
		if site := s.fnObjs[fn]; site != nil && fn.Kind != FnTranslated && fn.Err == nil {
			fn.Err = fmt.Errorf(fmtEndlessChain, ErrInternal, site.label)
		}
	}
}
