package eval

import (
	"maps"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// closure is a function value with its captured names (EVALUATION.md §4.1).
type closure struct {
	lam   syntax.Expr
	fn    check.Object
	vars  map[check.Object]value.Value
	self  value.Value
	it    value.Value
	short value.Value
	file  *syntax.File
	pkg   string
	t     types.Type
	p     *value.Prov
}

func (c *closure) Type() types.Type { return c.t }

func (c *closure) Prov() *value.Prov { return c.p }

// CanonText is empty: a function value cannot be formatted (E4503, static).
func (c *closure) CanonText() string { return "" }

// evalLambda creates a closure: one node, the lambda creation (EVALUATION.md §12.1).
func evalLambda(r *run, e syntax.Expr, _ *vpath) value.Value {
	c := &closure{
		lam: e, vars: map[check.Object]value.Value{}, self: r.fr.self, it: r.fr.it,
		short: r.fr.short, file: r.fr.file, pkg: r.fr.pkg, t: r.typeOf(e), p: r.prov(e, value.ProvComputed),
	}
	for _, obj := range r.ev.freeNames(e) {
		if v, ok := r.fr.vars[obj]; ok {
			c.vars[obj] = v
		}
	}
	return c
}

// freeNames are the locals and parameters a lambda's body names, found once per lambda.
func (e *Evaluator) freeNames(lam syntax.Expr) []check.Object {
	if names, ok := e.frees[lam]; ok {
		return names
	}
	seen := map[check.Object]bool{}
	var names []check.Object
	syntax.Inspect(lam, func(n syntax.Node) bool {
		id, ok := n.(*syntax.IdentExpr)
		if !ok {
			return true
		}
		obj := e.info.Uses[id]
		if obj != nil && !seen[obj] && (obj.Kind() == check.ObjLocal || obj.Kind() == check.ObjParam) {
			seen[obj] = true
			names = append(names, obj)
		}
		return true
	})
	e.frees[lam] = names
	return names
}

// invoke calls a function value (EVALUATION.md §12.1, DECISIONS 185).
func (r *run) invoke(fn value.Value, args []value.Value, site source.Span) value.Value {
	c, ok := fn.(*closure)
	if !ok {
		r.bug(nil)
		return nil
	}
	if ft, isFn := c.t.Base().(*types.FuncType); isFn && len(ft.Params) == len(args) {
		for i := range args {
			if args[i] = r.coerce(args[i], ft.Params[i], nil); args[i] == nil {
				return nil
			}
		}
	}
	if c.fn != nil {
		full := args
		if d, isFn := c.fn.Decl().(*syntax.FnDecl); isFn && len(args) < len(d.Params) {
			full = append(append([]value.Value(nil), args...), make([]value.Value, len(d.Params)-len(args))...)
		}
		return r.invokeFn(fnCall{obj: c.fn, args: full, site: site})
	}
	if !r.enter(site) {
		return nil
	}
	fr := &frame{
		vars: make(map[check.Object]value.Value, len(c.vars)+len(args)), self: c.self, it: c.it,
		short: c.short, file: c.file, pkg: c.pkg, call: site, caller: r.fr,
	}
	maps.Copy(fr.vars, c.vars)
	body := r.bindLambda(c, fr, args)
	if body == nil {
		r.bug(nil)
		return nil
	}
	saved := r.fr
	r.fr = fr
	r.depth++
	v := r.eval(body)
	r.fr = saved
	r.depth--
	return v
}

// bindLambda binds a lambda's parameters or a shorthand's receiver (TYPES.md §12.4).
func (r *run) bindLambda(c *closure, fr *frame, args []value.Value) syntax.Expr {
	switch lam := c.lam.(type) {
	case *syntax.ShorthandLambda:
		if len(args) != 1 {
			return nil
		}
		fr.short = args[0]
		return lam.Body
	case *syntax.LambdaExpr:
		if p, isPair := onlyPair(args); isPair && len(lam.Params) == pairNames {
			args = []value.Value{p.A, p.B}
		}
		if len(args) != len(lam.Params) {
			return nil
		}
		for i, id := range lam.Params {
			if obj := r.ev.info.Defs[id]; obj != nil {
				fr.vars[obj] = args[i]
			}
		}
		return lam.Body
	}
	return nil
}

func onlyPair(args []value.Value) (*value.Pair, bool) {
	if len(args) != 1 {
		return nil, false
	}
	p, ok := args[0].(*value.Pair)
	return p, ok
}
