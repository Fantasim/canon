package typedef

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// typeFunc is a `match`-bodied type function (VIEWMODEL.md 12.3 typeFunction): its parameters, the
// path its `match` reads below its parameter, its branches in `match` order, and the discriminant
// of every entry of each collection whose entries the drivers' program passes to it (J14).
func (s *Types) typeFunc(fn *types.TypeFunc) vm.TypeDef {
	def := vm.TypeDef{Kind: defTypeFunc, Name: fn.Name, Drivers: map[string]map[string]string{}}
	for _, p := range fn.Params {
		def.Params = append(def.Params, vm.Param{Name: p.Name, Type: s.Expr(nil, p.Type)})
	}
	sel := make([]string, len(fn.Scrutinee.Path))
	for i, f := range fn.Scrutinee.Path {
		sel[i] = f.Name
	}
	selected := strings.Join(sel, dot)
	def.Select = &selected
	for _, a := range fn.Arms {
		def.Branches = append(def.Branches, vm.Branch{Match: armMatch(fn.Scrutinee.Type, a), Type: s.Expr(nil, a.Result)})
	}
	w := s.drivers()
	if w == nil {
		return def
	}
	wfn := s.inWorld(w, fn)
	if wfn == nil {
		return def
	}
	for _, coll := range s.passed(w.Program, wfn) {
		if d := discriminants(w.Colls.Entries(coll), wfn.Scrutinee); len(d) > 0 {
			def.Drivers[encode.CollectionID(coll)] = d
		}
	}
	return def
}

// armMatch are the members an arm covers, by name (`false`, `true` for a Bool), or `_`.
func armMatch(scrutinee types.Type, a *types.TypeArm) []string {
	if a.Wildcard {
		return []string{wildcard}
	}
	e, isEnum := scrutinee.Base().(*types.EnumType)
	out := make([]string, 0, len(a.Members))
	for _, i := range a.Members {
		switch {
		case isEnum && i < len(e.Members):
			out = append(out, e.Members[i].Name)
		case !isEnum:
			out = append(out, strconv.FormatBool(i == 1))
		}
	}
	return out
}

// discriminants map each entry's key to the member the scrutinee's path reads (J14); an entry
// that is not of the parameter's record, or whose path crosses a ref or ends on none, has none.
func discriminants(entries []*value.Record, sc *types.Scrutinee) map[string]string {
	out := map[string]string{}
	want := recordOf(sc.Param.Type)
	for _, e := range entries {
		if e.Ident == nil || want != nil && recordOf(e.T) != want {
			continue
		}
		var v value.Value = e
		for _, f := range sc.Path {
			r, ok := v.(*value.Record)
			if !ok || f.Index >= len(r.Fields) {
				v = nil
				break
			}
			v = r.Fields[f.Index]
		}
		switch x := v.(type) {
		case *value.Member:
			out[e.Ident.Key.Text()] = x.CanonText()
		case *value.Bool:
			out[e.Ident.Key.Text()] = x.CanonText()
		}
	}
	return out
}

// recordOf is the record a value of t is, a ref's target entries included; nil for another type.
func recordOf(t types.Type) *types.RecordType {
	switch x := t.Base().(type) {
	case *types.RefType:
		return recordOf(x.Target.Elem)
	case *types.AppliedRecord:
		return x.Rec
	case *types.RecordType:
		return x
	}
	return nil
}

// argTarget is the collection a type argument's value is a ref into, nil for another value or
// a collection of an enclosing record, whose entries are per instance.
func argTarget(a *types.Arg, binder *types.Collection) *types.Collection {
	if a == nil {
		return nil
	}
	var t types.Type
	switch {
	case len(a.Path) > 0:
		t = a.Path[len(a.Path)-1].Type
	case a.Source == types.ArgParam && a.Param != nil:
		t = a.Param.Type
	case a.Source == types.ArgKey && binder != nil:
		t = &types.RefType{Target: binder}
	default:
		return nil
	}
	r, ok := shape.StripOptional(t).Base().(*types.RefType)
	if !ok || r.Target.Kind == types.CollField {
		return nil
	}
	return r.Target
}
