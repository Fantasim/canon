package eval

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// noteDefault keeps v, field f's default stored by the load being decoded, when it may hold a ref (EVALUATION.md §3.4).
func (e *Evaluator) noteDefault(v value.Value, f *types.Field) {
	ld := e.loading
	if ld == nil || !e.mayHoldRef(f.Type, nil) {
		return
	}
	if ld.defaults == nil {
		ld.defaults = map[value.Value]bool{}
	}
	ld.defaults[v] = true
}

// bindLoaded is v, site's decoded value, its defaults' refs bound as each instance completes (EVALUATION.md §3.4).
func (e *Evaluator) bindLoaded(v value.Value, site *loadSite) value.Value {
	if v == nil || site == nil || site.defaults == nil {
		return v
	}
	stack := []binding{{v: v, kids: e.decodedKids(v, site)}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.next < len(top.kids) {
			k := top.kids[top.next]
			top.next++
			stack = append(stack, binding{v: k, kids: e.decodedKids(k, site)})
			continue
		}
		if rec, ok := top.v.(*value.Record); ok {
			// A decoded record's defaults may share another's entries (`b: I = self.a`), so its refs
			// bind through the held entry, as a spread literal's do (DECISIONS 497, 498).
			e.bindRefs(rec, true)
		}
		stack = stack[:len(stack)-1]
	}
	return v
}

// decodedKids are the composites v holds that the decoder built and that may hold a ref; a
// default's value is not one: the binder of the instance holding it copies what it binds there.
func (e *Evaluator) decodedKids(v value.Value, site *loadSite) []value.Value {
	var out []value.Value
	for _, k := range components(v) {
		if isComposite(k) && !site.defaults[k] && e.mayHoldRef(k.Type(), nil) {
			out = append(out, k)
		}
	}
	return out
}
