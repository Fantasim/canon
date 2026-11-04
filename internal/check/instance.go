package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// RecordParams are the objects of the record decl's own value parameters; nil for a case or a record without any (TYPES.md §11.1).
func RecordParams(info *Info, decl types.Type) map[Object]bool {
	r, ok := decl.(*types.RecordType)
	if !ok || r.Decl == nil || len(r.Decl.Params) == 0 {
		return nil
	}
	out := map[Object]bool{}
	for _, po := range paramObjects(info, r) {
		out[po] = true
	}
	return out
}

// paramObjects are the parameter objects of a record, from its declaration.
func paramObjects(info *Info, r *types.RecordType) []*object {
	var out []*object
	for _, p := range r.Decl.Params {
		if po, ok := info.Defs[p.Name].(*object); ok {
			out = append(out, po)
		}
	}
	return out
}

// ReadsInstance reports a default reading its instance: a field, `self` or one of params, not a lambda's own (TYPES.md §3.3, §15).
func ReadsInstance(info *Info, e syntax.Expr, params map[Object]bool) bool {
	found := false
	syntax.Inspect(e, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.SelfExpr:
			found = true
		case *syntax.IdentExpr:
			if o := info.Uses[x]; o != nil && (o.Kind() == ObjField || params[o]) {
				found = true
			}
		}
		return !found
	})
	return found
}
