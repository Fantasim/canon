package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// accept is `e ⇐ want` once e is typed s (TYPES.md §6.2).
func (c *checker) accept(env *env, e syntax.Expr, s, want types.Type) {
	e = inner(e)
	if s.Kind() == types.Error || want.Kind() == types.Error {
		return
	}
	conv, ok := c.convert(s, want)
	if !ok && isIntLiteral(e) && floatTarget(want) {
		conv, ok = intToFloat(s, want), true
	}
	if !ok {
		c.mismatch(env, e, s, want)
		return
	}
	if conv != nil {
		c.info.Conv[e] = conv
	}
	c.checkLiteral(env, e, want)
}

// mismatch reports why s is not assignable to want: E3403 for an optional (or none) where its
// present form would do, E3311 for an Int where a Float is expected, else E3002.
func (c *checker) mismatch(env *env, e syntax.Expr, s, want types.Type) {
	sb := s.Base()
	switch {
	case sb.Kind() == types.DepUnion || sb.Kind() == types.TypeApp:
		c.report(env, diag.E3804.At(env.span(e), want.String(), depName(s)))
	case sb.Kind() == types.None:
		c.report(env, diag.E3403.At(env.span(e), want))
	case sb.Kind() == types.Optional && c.assignable(sb.(*types.OptionalType).Elem, want):
		c.report(env, diag.E3403.At(env.span(e), want))
	case sb.Kind() == types.Int && want.Base().Kind() == types.Float:
		c.report(env, diag.E3311.At(env.span(e), env.span(e)))
	default:
		c.report(env, diag.E3002.At(env.span(e), want, s))
	}
}

// assignable is S ≤ E for the checker, refs resolved.
func (c *checker) assignable(s, want types.Type) bool {
	_, ok := c.convert(s, want)
	return ok
}

// isIntLiteral reports an integer literal token, parenthesized or not (TYP-10).
func isIntLiteral(e syntax.Expr) bool {
	for {
		switch x := e.(type) {
		case *syntax.IntLit:
			return true
		case *syntax.ParenExpr:
			e = x.X
		default:
			return false
		}
	}
}

// floatTarget reports Float or Float? as an expected type.
func floatTarget(want types.Type) bool {
	return unwrap(want).Kind() == types.Float
}

// intToFloat is the conversion of an integer literal to Float, wrapped when want is Float?.
func intToFloat(s, want types.Type) *Conversion {
	f := &Conversion{Kind: ConvIntLitToFloat, From: s, To: unwrap(want)}
	if want.Base().Kind() == types.Optional {
		return &Conversion{Kind: ConvWrap, From: s, To: want, Inner: f}
	}
	return f
}

// convert is S ≤ E with its conversion (TYPES.md §6.2).
func (c *checker) convert(s, want types.Type) (*Conversion, bool) {
	sb, wb := s.Base(), want.Base()
	switch {
	case sb.Kind() == types.Error || wb.Kind() == types.Error || sb.Kind() == types.Never:
		return nil, true
	case sb.Kind() == types.DepUnion || sb.Kind() == types.TypeApp:
		return nil, types.Assignable(sb, wb)
	case wb.Kind() == types.Any || wb.Kind() == types.DepUnion || wb.Kind() == types.TypeApp:
		return nil, true
	case types.Identical(sb, wb):
		return nil, true
	case wb.Kind() == types.Optional:
		return c.convertOptional(s, sb, want, wb.(*types.OptionalType))
	case sb.Kind() == types.Optional || sb.Kind() == types.None:
		return nil, false
	case sb.Kind() == types.Ref && wb.Kind() != types.Ref && types.Identical(c.coll(sb.(*types.RefType)).Elem, wb):
		return &Conversion{Kind: ConvDeref, From: s, To: want}, true
	}
	return c.convertComposite(s, sb, want, wb)
}

// convertOptional is None ≤ T?, S? ≤ T? (on a present value) and S ≤ T? (wraps).
func (c *checker) convertOptional(s, sb, want types.Type, wo *types.OptionalType) (*Conversion, bool) {
	switch x := sb.(type) {
	case *types.OptionalType:
		inner, ok := c.convert(x.Elem, wo.Elem)
		if !ok || inner == nil {
			return nil, ok
		}
		return &Conversion{Kind: ConvPresent, From: s, To: want, Inner: inner}, true
	}
	if sb.Kind() == types.None {
		return nil, true
	}
	inner, ok := c.convert(s, wo.Elem)
	if !ok {
		return nil, false
	}
	return &Conversion{Kind: ConvWrap, From: s, To: want, Inner: inner}, true
}

// convertComposite is the rows of §6.2 for refs, variants, lists, maps, pairs and unions.
func (c *checker) convertComposite(s, sb, want, wb types.Type) (*Conversion, bool) {
	switch w := wb.(type) {
	case *types.RefType:
		if sb.Kind() != types.Ref && types.Identical(sb, c.coll(w).Elem) {
			return &Conversion{Kind: ConvEntryToRef, From: s, To: want}, true
		}
	case *types.VariantType:
		if ct, ok := sb.(*types.CaseType); ok && ct.Variant == w {
			return &Conversion{Kind: ConvCaseToVariant, From: s, To: want}, true
		}
	case *types.ListType:
		return c.convertList(s, sb, want, w)
	case *types.MapType:
		return c.convertMap(s, sb, want, w)
	case *types.DepMapType:
		return c.convertDepMap(s, sb, want, w)
	case *types.PairType:
		return c.convertPair(s, sb, want, w)
	case *types.LitUnionType:
		return c.convert(s, w.Of)
	case *types.FuncType:
		f, ok := sb.(*types.FuncType)
		return nil, ok && types.Assignable(f, w) && c.assignable(f.Result, w.Result)
	}
	return nil, false
}

// convertList is `[S] ≤ [T]` element-wise, a keyed list or a table to a plain list keeping
// identities, and a list to a keyed list (its keys checked at evaluation, E3102).
func (c *checker) convertList(s, sb, want types.Type, w *types.ListType) (*Conversion, bool) {
	switch from := sb.(type) {
	case *types.TableType:
		if w.KeyedBy == nil && types.Identical(from.Elem, w.Elem) {
			return &Conversion{Kind: ConvToList, From: s, To: want}, true
		}
	case *types.ListType:
		inner, ok := c.convert(from.Elem, w.Elem)
		switch {
		case !ok:
			return nil, false
		case inner != nil || (w.KeyedBy != nil && from.KeyedBy != w.KeyedBy):
			return &Conversion{Kind: ConvElements, From: s, To: want, Inner: inner}, true
		case from.KeyedBy != nil && w.KeyedBy == nil:
			return &Conversion{Kind: ConvToList, From: s, To: want}, true
		}
		return nil, true
	}
	return nil, false
}

func (c *checker) convertMap(s, sb, want types.Type, w *types.MapType) (*Conversion, bool) {
	from, ok := sb.(*types.MapType)
	if !ok {
		return nil, false
	}
	key, okK := c.convert(from.Key, w.Key)
	val, okV := c.convert(from.Value, w.Value)
	if !okK || !okV {
		return nil, false
	}
	if key == nil && val == nil {
		return nil, true
	}
	return &Conversion{Kind: ConvElements, From: s, To: want, Key: key, Inner: val}, true
}

// convertDepMap is `{K: V} ≤` a dependent map over `ref C` when `K ≤ ref C`; each value is
// checked at evaluation against its computed type (E3802).
func (c *checker) convertDepMap(s, sb, want types.Type, w *types.DepMapType) (*Conversion, bool) {
	from, ok := sb.(*types.MapType)
	if !ok {
		return nil, false
	}
	key, okK := c.convert(from.Key, &types.RefType{Target: w.Coll})
	if !okK {
		return nil, false
	}
	return &Conversion{Kind: ConvElements, From: s, To: want, Key: key}, true
}

func (c *checker) convertPair(s, sb, want types.Type, w *types.PairType) (*Conversion, bool) {
	from, ok := sb.(*types.PairType)
	if !ok {
		return nil, false
	}
	a, okA := c.convert(from.A, w.A)
	b, okB := c.convert(from.B, w.B)
	if !okA || !okB {
		return nil, false
	}
	if a == nil && b == nil {
		return nil, true
	}
	return &Conversion{Kind: ConvElements, From: s, To: want, Key: a, Inner: b}, true
}
