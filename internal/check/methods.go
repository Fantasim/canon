package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// builtinMethod calls a built-in method on a receiver of type recv (STDLIB.md §4 to §7, §10).
func (c *checker) builtinMethod(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, recv, want types.Type) types.Type {
	name := s.Name.Name
	b := newBinding()
	rows := c.methodRows(recv, s.X, name, b)
	if len(rows) == 0 {
		if recv.Kind() == types.DepUnion {
			c.report(env, diag.E3804.At(env.span(s.Name), name, depName(recv)))
		} else {
			c.report(env, diag.E3003.At(env.span(s.Name), recv, diag.KindMethod, name))
		}
		c.argsAlone(env, x)
		return types.ErrorType
	}
	if c.misplacedArgs(env, x) {
		return types.ErrorType
	}
	if s.X != nil {
		b.from(0, s.X)
	}
	bc := &builtinCall{x: x, fun: s, name: name, kind: CalleeBuiltin, b: b, want: want}
	if !c.receiverFits(env, bc, s, recv, rows[0].sig) {
		c.argsAlone(env, x)
		return types.ErrorType
	}
	if t, handled := c.keyedMethod(env, bc, rows); handled {
		return t
	}
	if name == methodMatches {
		return c.matches(env, bc, rows[0])
	}
	return c.applyRows(env, bc, rows)
}

// methodRows are the rows of name for a receiver of type t, with its parameters bound.
func (c *checker) methodRows(t types.Type, recvExpr syntax.Expr, name string, b *binding) []row {
	switch x := t.Base().(type) {
	case *types.MapType:
		b.bind(tK, x.Key)
		b.bind(tV, staticView(x.Value))
		return rowsNamed(mapMethods, name)
	case *types.DepMapType:
		b.bind(tK, &types.RefType{Target: x.Coll})
		b.bind(tV, staticView(x.Value))
		return rowsNamed(mapMethods, name)
	case *types.ListType:
		b.bind(tT, x.Elem)
		if x.KeyedBy == nil {
			return append(rowsNamed(seqMethods, name), rowsNamed(listMethods, name)...)
		}
		c.bindKeyed(b, x.KeyedBy.Type, recvExpr)
		return append(rowsNamed(seqMethods, name), withoutActive(rowsNamed(keyedMethods, name))...)
	case *types.TableType:
		b.bind(tT, x.Elem)
		c.bindKeyed(b, types.StringType, recvExpr)
		return append(rowsNamed(seqMethods, name), rowsNamed(keyedMethods, name)...)
	}
	switch t.Base().Kind() {
	case types.String:
		return rowsNamed(stringMethods, name)
	case types.Range:
		return rowsNamed(rangeMethods, name)
	default:
	}
	return nil
}

// bindKeyed binds a keyed collection's key type and, when the receiver names a collection,
// the ref into it that keys() returns.
func (c *checker) bindKeyed(b *binding, key types.Type, recvExpr syntax.Expr) {
	b.bind(keyT, key)
	if recvExpr == nil {
		return
	}
	if coll := c.receiverColl(recvExpr); coll != nil {
		b.bind(refT, &types.RefType{Target: coll})
	}
}

// withoutActive drops active(), which only tables have.
func withoutActive(rows []row) []row {
	var out []row
	for _, r := range rows {
		if r.sig.name != methodActive {
			out = append(out, r)
		}
	}
	return out
}

// receiverFits checks what a method asks of its receiver's elements: numbers for sum, min and
// max, strings for join, lists for flatten (E3002 otherwise, E3804 for dependent elements).
func (c *checker) receiverFits(env *env, bc *builtinCall, s *syntax.SelectorExpr, recv types.Type, sig bsig) bool {
	elem, bound := bc.b.vars[tT]
	if !bound {
		return true
	}
	if sig.name == methodFlatten {
		inner, isList := elem.Base().(*types.ListType)
		if !isList {
			c.report(env, diag.E3002.At(env.span(s), listT(listT(elem)), recv))
			return false
		}
		bc.b.bind(tU, inner.Elem)
		return true
	}
	if sig.elemCons != consNone && !satisfies(elem, sig.elemCons) {
		if !c.notDependent(env, s, elem, sig.name) {
			c.report(env, diag.E3002.At(env.span(s), listT(consExample(sig.elemCons)), recv))
		}
		return false
	}
	return true
}

// consExample is a type that meets a constraint, for the expected side of E3002.
func consExample(k constraint) types.Type {
	switch k {
	case consString, consKey:
		return types.StringType
	case consEq:
		return types.AnyType
	default:
		return types.IntType
	}
}

// keyedMethod handles the methods of a keyed collection that take an element or a key (STDLIB.md §5).
func (c *checker) keyedMethod(env *env, bc *builtinCall, rows []row) (types.Type, bool) {
	key, keyed := bc.b.vars[keyT]
	if !keyed || len(bc.x.Args) != 1 || bc.x.Args[0].Name != nil {
		return nil, false
	}
	arg := bc.x.Args[0].Value
	elem := bc.b.vars[tT]
	switch bc.name {
	case methodContains, methodIndexOf:
		c.memberOrKey(env, arg, bc.x.Fun.(*syntax.SelectorExpr).X, elem, key)
	case methodGet, methodFind:
		c.keyArg(env, arg, elem, key, bc.x)
	default:
		return nil, false
	}
	return c.finishBuiltin(bc, rows[0]), true
}

// keyArg types the key given to get or find: the key type or a ref into the collection.
func (c *checker) keyArg(env *env, arg syntax.Expr, elem, key types.Type, x *syntax.CallExpr) {
	if c.bareKey(env, arg, x.Fun.(*syntax.SelectorExpr).X, key) {
		return
	}
	t := c.synth(env, arg)
	if t.Kind() == types.Error || c.assignable(t, key) {
		return
	}
	if r, isRef := t.Base().(*types.RefType); isRef && types.Identical(c.coll(r).Elem, elem) {
		return
	}
	c.report(env, diag.E3002.At(env.span(arg), key, t))
}

// matches is `s.matches(re)`: re must be a regex literal (STDLIB.md §7, §8).
func (c *checker) matches(env *env, bc *builtinCall, r row) types.Type {
	if _, fits := argMap(bc.x, r.sig); !fits {
		c.arityError(env, bc, r.sig)
		c.argsAlone(env, bc.x)
		return types.ErrorType
	}
	arg := bc.x.Args[0].Value
	if _, ok := arg.(*syntax.RegexLit); !ok {
		if t := c.synth(env, arg); t.Kind() != types.Error {
			c.report(env, diag.E3007.AtBinary(env.span(bc.x), methodMatches, types.StringType, t))
		}
		return types.ErrorType
	}
	c.info.Types[arg] = types.StringType
	return c.finishBuiltin(bc, r)
}
