package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// readsChanged reports f's default (whose record's parameters stay bound) or its type's arguments reading a field changed marks (TYPES.md §11.1, §15).
func (e *Evaluator) readsChanged(f *types.Field, fields []*types.Field, changed []bool) bool {
	marked := func(j int) bool { return j < len(changed) && changed[j] }
	return slices.ContainsFunc(f.DependsOn, marked) || slices.ContainsFunc(e.Reads(f, fields), marked)
}

// omitter is the literal or object that omitted rec's field j, which its default's value came via (EVALUATION.md §13).
func omitter(rec *value.Record, j int) *value.Prov {
	if v := rec.Fields[j]; v != nil {
		if p := v.Prov(); p != nil && p.Kind == value.ProvDefault && p.Via != nil {
			return p.Via
		}
	}
	return rec.P
}
