package typedef

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// passed are the collections, in the order met, whose entries prog, the drivers', passes to fn's
// discriminating parameter: the target of the ref an argument reads (VIEWMODEL.md J14).
func (s *Types) passed(prog *check.Program, fn *types.TypeFunc) []*types.Collection {
	if s.applied == nil {
		sc := &scanner{
			ctx: s.ctx, applied: map[*types.TypeFunc][]*types.Collection{}, seen: map[recordKey]bool{},
			keys: map[*types.Collection]*types.Param{},
		}
		for _, p := range prog.Packages {
			for _, o := range p.Decls {
				sc.decl(o)
			}
		}
		s.applied = sc.applied
	}
	return s.applied[fn]
}

// scanner finds the applications of type functions in the program's types: through functions
// without a `match` (expanded, J11) and parameterized records (their arguments bound).
type scanner struct {
	keys    map[*types.Collection]*types.Param // one stand-in per map key passed down (resolved)
	ctx     context.Context                    // a cancelled scan stops; Build then returns the context's error
	applied map[*types.TypeFunc][]*types.Collection
	seen    map[recordKey]bool
}

// recordKey is an applied record walked once: the record and what its arguments pass, never
// their whole paths, so a record applying itself down a ref still finishes.
type recordKey struct {
	rec  *types.RecordType
	args string
}

// decl walks the fields of a record or variant declaration, or a let's type.
func (sc *scanner) decl(o check.Object) {
	if o.Type() == nil {
		return
	}
	for _, f := range declaredFields(o) {
		sc.walk(f.Type, nil)
	}
	if o.Kind() == check.ObjLet {
		sc.walk(o.Type(), nil)
	}
}

// declaredFields are the fields of a record declaration, or of every case of a variant one.
func declaredFields(o check.Object) []*types.Field {
	if o.Kind() != check.ObjTypeName {
		return nil
	}
	v, ok := shape.Unalias(o.Type()).(*types.VariantType)
	if !ok {
		return encode.FieldsOf(shape.Unalias(o.Type()))
	}
	var out []*types.Field
	for _, c := range v.Cases {
		out = append(out, c.Fields...)
	}
	return out
}

// walk records each application t holds; binder is the collection of the enclosing dependent
// map, whose key an argument may read.
func (sc *scanner) walk(t types.Type, binder *types.Collection) {
	encode.Walk(t, func(x types.Type) bool {
		switch y := x.Base().(type) {
		case *types.DepMapType:
			sc.walk(y.Value, y.Coll)
			return false
		case *types.TypeAppType:
			sc.app(y, binder)
			return false
		case *types.AppliedRecord:
			sc.record(y, binder)
			return false
		}
		return true
	})
}

// app records the collection a `match` function's application passes, then walks its branches
// with its arguments; any other function's application is walked expanded (J11).
func (sc *scanner) app(app *types.TypeAppType, binder *types.Collection) {
	fn := app.Fn
	args := sc.resolved(app.Args, binder)
	if !control.Matches(fn) {
		if fn.Body != nil {
			sc.walk(shape.Substitute(fn.Body, fn.Params, args), binder)
		}
		return
	}
	if coll := argTarget(control.Scrutinized(app), binder); coll != nil && !slices.Contains(sc.applied[fn], coll) {
		sc.applied[fn] = append(sc.applied[fn], coll)
	}
	for _, a := range fn.Arms {
		sc.walk(shape.Substitute(a.Result, fn.Params, args), binder)
	}
}

// record walks an applied record's fields with its arguments bound, once per arguments.
func (sc *scanner) record(a *types.AppliedRecord, binder *types.Collection) {
	k := recordKey{rec: a.Rec, args: argsKey(a.Args, binder)}
	if sc.seen[k] || sc.ctx.Err() != nil {
		return
	}
	sc.seen[k] = true
	args := sc.resolved(a.Args, binder)
	for _, f := range a.Rec.Fields {
		sc.walk(shape.Substitute(f.Type, a.Rec.Params, args), binder)
	}
}

// resolved are args with each one reading the enclosing map's key bound, before they are passed
// down: a stand-in parameter holding a ref into binder, one per collection, so that a map inside
// the body, whatever its key's name, never takes it for its own (J13, J14).
func (sc *scanner) resolved(args []*types.Arg, binder *types.Collection) []*types.Arg {
	out := make([]*types.Arg, len(args))
	for i, a := range args {
		out[i] = a
		if a.Source != types.ArgKey || binder == nil {
			continue
		}
		p, ok := sc.keys[binder]
		if !ok {
			p = &types.Param{Name: a.Binder, Type: &types.RefType{Target: binder}}
			sc.keys[binder] = p
		}
		out[i] = &types.Arg{Source: types.ArgParam, Param: p, Path: a.Path}
	}
	return out
}

// argsKey tells arguments apart by what drivers read of them: their source, their parameter
// and the collection their value is a ref into (J14); one set of keys per record, so finite.
func argsKey(args []*types.Arg, binder *types.Collection) string {
	var b strings.Builder
	for _, a := range args {
		fmt.Fprintf(&b, fmtArgKey, a.Source, a.Param, argTarget(a, binder))
	}
	return b.String()
}
