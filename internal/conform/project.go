package conform

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// selfFn is the value of recv's precomputed method name, nil when it has none; its error ends Fill.
type selfFn func(recv value.Value, name string) (value.Value, error)

// project is recv's value at each path the body reads, and their key (CONFORMANCE.md §6.1); ErrUnsupported breaks DECISIONS 204's IR invariant.
func project(recv value.Value, reads []*ir.Read, method selfFn) ([]value.Value, string, error) {
	out := make([]value.Value, len(reads))
	for i, r := range reads {
		v, err := read(recv, r.Path, method)
		if err != nil {
			return nil, "", err
		}
		if v == nil {
			return nil, "", ErrUnsupported
		}
		out[i] = v
	}
	key, ok := keysOf(out)
	if !ok {
		return nil, "", ErrUnsupported
	}
	return out, key, nil
}

// read is recv's value at path: fields, or a precomputed method of self read like a field (CONFORMANCE.md §2.2; meta/decisions/log-2026-09-24.md).
func read(recv value.Value, path []string, method selfFn) (value.Value, error) {
	v := recv
	for _, name := range path {
		// a path through an absent optional reads none
		if _, none := v.(*value.None); none {
			break
		}
		v = field(v, name)
	}
	if v != nil || len(path) != 1 {
		return v, nil
	}
	return method(recv, path[0])
}

// field is a record or case value's field name; nil when v is none of these, or the field an
// input, which holds no value at build time.
func field(v value.Value, name string) value.Value {
	rec, ok := v.(*value.Record)
	if !ok {
		return nil
	}
	var fields []*types.Field
	switch d := rec.T.Base().(type) {
	case *types.RecordType:
		fields = d.Fields
	case *types.AppliedRecord:
		fields = d.Rec.Fields
	case *types.CaseType:
		fields = d.Fields
	}
	for i, f := range fields {
		if f.Name == name && i < len(rec.Fields) {
			return rec.Fields[i]
		}
	}
	return nil
}
