package jsongen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/wire"
)

// Generate is the json generator (ir.Generator): a file per value, `$fns` in the first (WIRE.md §8).
func Generate(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	if p == nil || e == nil || e.Target != ir.TargetJSON {
		return nil, ErrEmit
	}
	values, err := selection(p, e)
	if err != nil {
		return nil, err
	}
	paths, err := layout(e, values)
	if err != nil {
		return nil, err
	}
	if err := dataMode(p, e, values, paths); err != nil {
		return nil, err
	}
	x, err := newIndex(p, values)
	if err != nil {
		return nil, err
	}
	files := make([]ir.File, len(values))
	for i, v := range values {
		var fns []*ir.ExportFn
		if i == 0 {
			fns = p.Fns
		}
		content, err := x.document(p.Name, v, fns)
		if err != nil {
			return nil, fmt.Errorf(fmtContext, v.Name, err)
		}
		files[i] = ir.File{Path: paths[i], Content: content}
	}
	return files, nil
}

// document is v's file; its `$schema` covers fns (decision 118) and is Value.Schema (decision 128).
func (x *index) document(pkg string, v *ir.Value, fns []*ir.ExportFn) ([]byte, error) {
	schema, err := ir.Schema(pkg, v.Name, &v.Type, fns)
	if err != nil {
		return nil, err
	}
	if v.Schema != "" && v.Schema != schema {
		return nil, fmt.Errorf(fmtSchema, ErrSchema, v.Schema, schema)
	}
	stored, err := packageFns(fns)
	if err != nil {
		return nil, err
	}
	x.err = nil
	doc := wire.Document{Schema: schema, Kind: v.Type.Kind, V: v.V, Fns: stored, Methods: x.dollars}
	b, err := doc.Encode()
	if err == nil {
		err = x.err
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}
