package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// bodyWheres checks the pending `where` predicates of body's field types now, once, before a fold builds body (DECISIONS 454).
func (c *checker) bodyWheres(body *recordCtx) {
	w := wherePreds{preds: map[syntax.Expr]bool{}, aliases: map[*types.Alias]bool{}, funcs: map[*types.TypeFunc]bool{}}
	for _, fo := range body.order {
		w.of(fo.field.Type)
	}
	if len(w.preds) == 0 {
		return
	}
	var now, rest []whereJob
	for _, j := range c.wheres {
		if w.preds[j.pred] {
			now = append(now, j)
		} else {
			rest = append(rest, j)
		}
	}
	c.wheres = rest
	for _, j := range now {
		c.checkWhere(j)
	}
}

// wherePreds collects the predicates a type holds, each named type and type function walked once.
type wherePreds struct {
	preds   map[syntax.Expr]bool
	aliases map[*types.Alias]bool
	funcs   map[*types.TypeFunc]bool
}

// of walks t down to its records: a record's own fields are its body's, checked when a fold builds it.
func (w *wherePreds) of(t types.Type) {
	switch x := t.(type) {
	case *types.Refined:
		if x.Where != nil {
			w.preds[x.Where.Expr] = true
		}
		w.of(x.Of)
	case *types.Alias:
		if !w.aliases[x] {
			w.aliases[x] = true
			w.of(x.Def)
		}
	case *types.OptionalType:
		w.of(x.Elem)
	case *types.ListType:
		w.of(x.Elem)
	case *types.MapType:
		w.of(x.Key)
		w.of(x.Value)
	case *types.DepMapType:
		w.of(x.Value)
	case *types.LitUnionType:
		w.of(x.Of)
	case *types.TypeAppType:
		w.typeFunc(x.Fn)
	case *types.DepUnionType:
		w.typeFunc(x.Fn)
	}
}

// typeFunc walks a type function's body and arms once.
func (w *wherePreds) typeFunc(f *types.TypeFunc) {
	if f == nil || w.funcs[f] {
		return
	}
	w.funcs[f] = true
	w.of(f.Body)
	for _, a := range f.Arms {
		w.of(a.Result)
	}
}
