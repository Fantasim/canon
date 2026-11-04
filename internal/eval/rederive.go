package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// rederive is field j's default re-derived: changed, the amendment's layer; else the one its value had (API.md §4).
func (r *run) rederive(rec *value.Record, j int, at *vpath, changed bool) value.Value {
	f := fieldsOf(rec.T)[j]
	if changed {
		return r.defaultValue(rec, f, at, rec.P)
	}
	layer := r.fr.layer
	r.fr.layer = layerOf(rec.Fields[j])
	defer func() { r.fr.layer = layer }()
	return r.defaultValue(rec, f, at, rec.P)
}

// layerOf is the layer of v's provenance, "" for none.
func layerOf(v value.Value) string {
	if v == nil {
		return ""
	}
	return v.Prov().Layer()
}

// readsChanged reports f's default (whose record's parameters stay bound) or its type's arguments reading a field changed marks (TYPES.md §11.1, §15).
func (e *Evaluator) readsChanged(f *types.Field, fields []*types.Field, changed []bool) bool {
	marked := func(j int) bool { return j < len(changed) && changed[j] }
	return slices.ContainsFunc(f.DependsOn, marked) || slices.ContainsFunc(e.Reads(f, fields), marked)
}
