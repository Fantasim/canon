package jsongen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// typeKey names a record, or a case of a variant, as both the IR and the values know it.
type typeKey struct {
	qname, kase string
}

// recvKey is one receiver of one method: an Instance is found by the receiver's pointer.
type recvKey struct {
	fn   *ir.ExportFn
	recv *value.Record
}

// index holds the stored methods of every record and case the emitted data can reach, and
// their per-receiver results; err is the first failure of the Methods callback.
type index struct {
	methods   map[typeKey][]*ir.ExportFn
	instances map[recvKey]*ir.Instance
	seen      map[ir.Type]bool
	err       error
}

// newIndex walks the package's types and what its values and fns reach, imported types included.
func newIndex(p *ir.Package, values []*ir.Value) (*index, error) {
	x := &index{methods: map[typeKey][]*ir.ExportFn{}, instances: map[recvKey]*ir.Instance{}, seen: map[ir.Type]bool{}}
	for _, t := range p.Types {
		x.named(t)
	}
	for _, v := range values {
		x.walk(&v.Type)
	}
	for _, fn := range p.Fns {
		x.walkFn(fn)
	}
	return x, x.err
}

func (x *index) walk(t *ir.TypeRef) {
	if t == nil {
		return
	}
	x.named(t.Named)
	x.walk(t.Elem)
	x.walk(t.Key)
}

func (x *index) walkFn(fn *ir.ExportFn) {
	for _, p := range fn.Params {
		x.walk(&p.Type)
	}
	x.walk(&fn.Result)
}

func (x *index) walkFields(fields []*ir.Field) {
	for _, f := range fields {
		x.walk(&f.Type)
	}
}

// named indexes a type once: a record's or each case's methods, then what its fields reach.
func (x *index) named(n ir.Type) {
	if n == nil || x.seen[n] {
		return
	}
	x.seen[n] = true
	switch t := n.(type) {
	case *ir.Record:
		x.register(typeKey{qname: t.QName()}, t.Methods)
		x.walkFields(t.Fields)
	case *ir.Variant:
		for _, c := range t.Cases {
			x.register(typeKey{qname: t.QName(), kase: c.Name}, c.Methods)
			x.walkFields(c.Fields)
		}
	case *ir.Dependent:
		for _, b := range t.Branches {
			x.walk(&b.Type)
		}
	}
}

// register indexes the stored methods, never the translated (WIRE.md §5.11), and their instances.
func (x *index) register(k typeKey, methods []*ir.ExportFn) {
	for _, m := range methods {
		if m.Kind == ir.FnTranslated {
			continue
		}
		x.methods[k] = append(x.methods[k], m)
		x.walkFn(m)
		for _, in := range m.Instances {
			key := recvKey{fn: m, recv: in.Recv}
			if in.Recv == nil || x.instances[key] != nil {
				x.fail(fmt.Errorf(fmtNamed, ErrFn, m.Name))
				continue
			}
			x.instances[key] = in
		}
	}
}

// dollars is the wire.Methods of the data: r's `$` keys, by receiver pointer (decision 128).
func (x *index) dollars(r *value.Record) []wire.Fn {
	methods := x.methods[keyOf(r.T)]
	out := make([]wire.Fn, 0, len(methods))
	for _, m := range methods {
		in := x.instances[recvKey{fn: m, recv: r}]
		if in == nil {
			x.fail(fmt.Errorf(fmtNamed, ErrFn, m.Name))
			return nil
		}
		fn, err := stored(m, in.Result, in.Table)
		if err != nil {
			x.fail(err)
			return nil
		}
		out = append(out, fn)
	}
	return out
}

func (x *index) fail(err error) {
	if x.err == nil {
		x.err = err
	}
}

// keyOf is the declaration a record value's type names; wire refuses any other shape.
func keyOf(t types.Type) typeKey {
	if t == nil {
		return typeKey{}
	}
	switch d := t.Base().(type) {
	case *types.RecordType:
		return typeKey{qname: d.Pkg + qnameSep + d.Name}
	case *types.AppliedRecord:
		return keyOf(d.Rec)
	case *types.CaseType:
		return typeKey{qname: d.Variant.Pkg + qnameSep + d.Variant.Name, kase: d.Name}
	default:
		return typeKey{}
	}
}

// packageFns is the `$fns` of a file: every package fn but the translated (WIRE.md §5.11).
func packageFns(fns []*ir.ExportFn) ([]wire.Fn, error) {
	var out []wire.Fn
	for _, fn := range fns {
		if fn.Kind == ir.FnTranslated {
			continue
		}
		w, err := stored(fn, fn.Value, fn.Table)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// stored is a fn's data: a precomputed result, or a lookup's dense table over its domains.
func stored(fn *ir.ExportFn, result value.Value, table *ir.LookupTable) (wire.Fn, error) {
	switch {
	case fn.Kind == ir.FnPrecomputed && result != nil:
		return wire.Fn{Name: fn.Name, Result: result}, nil
	case fn.Kind == ir.FnLookup && table != nil && len(table.Domains) > 0:
		return wire.Fn{Name: fn.Name, Domains: table.Domains, Cells: table.Cells}, nil
	default:
		return wire.Fn{}, fmt.Errorf(fmtNamed, ErrFn, fn.Name)
	}
}
