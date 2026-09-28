package ir

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// checkVariantMembers is E8019 `VariantMethod`: types.VariantType holds no method, so no generator writes a variant-level `export fn` (TYPES.md §12.1, DECISIONS 226), nor a translated call of one; plain methods and checks are never emitted, so never refused (decision 37).
func (s *stage) checkVariantMembers(u *unit, es *emitSite) {
	for _, t := range u.p.Types {
		if v, ok := t.(*Variant); ok {
			for _, span := range s.members[v] {
				u.reportGenConstruct(es, span, diag.KindVariantMethod)
			}
		}
	}
	for _, span := range u.variantCalls {
		u.reportGenConstruct(es, span, diag.KindVariantMethod)
	}
}

// variantMembers records a variant's methods written outside its cases, and locates its export fns at their names; a broken one draws no stage-E finding (decision 213).
func (s *stage) variantMembers(f *syntax.File, items []syntax.VariantItem) []source.Span {
	var out []source.Span
	for _, it := range items {
		d, ok := it.(*syntax.FnDecl)
		if !ok {
			continue
		}
		s.variantFns[d] = true
		if f != nil && exported(d) && !s.info.Broken[s.info.Defs[d.Name]] {
			out = append(out, f.Span(d.Name))
		}
	}
	return out
}

// variantCall refuses a call the portable subset admits but no generator writes: a variant-level precomputed export fn of self, from a case's translated method (CONFORMANCE.md §2.2); it is E8019 per emit, not E9001. A call inside a construct refused already adds nothing.
func (t *translator) variantCall(x *syntax.CallExpr, c *check.Callee) bool {
	if c.Kind != check.CalleeMethod || c.Obj == nil {
		return false
	}
	d, ok := c.Obj.Decl().(*syntax.FnDecl)
	if !ok || !t.s.variantFns[d] || !exported(d) || len(d.Params) != 0 || !onSelf(x) {
		return false
	}
	if t.inside == 0 {
		t.u.variantCalls = append(t.u.variantCalls, t.file.Span(x))
	}
	t.refused = true
	return true
}
