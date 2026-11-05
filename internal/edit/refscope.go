package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// scopeName is the value bare key token x names in scope, not a key of its text (DECISIONS 318).
func (s *Snapshot) scopeName(x *syntax.IdentExpr) check.Object {
	if o := s.info.Uses[x]; o != nil && scopeKinds[o.Kind()] {
		return o
	}
	return nil
}

// scopedSegs reports a segment of segs keyed by a name resolved in scope: written by its text, the
// line's path names no value statically (DECISIONS 318).
func (s *Snapshot) scopedSegs(segs []*syntax.AmendSegment) bool {
	return slices.ContainsFunc(segs, func(seg *syntax.AmendSegment) bool {
		x, isIdent := syntax.Unparen(seg.Key).(*syntax.IdentExpr)
		return seg.Key != nil && isIdent && s.scopeName(x) != nil
	})
}

// inScope reports x resolved in scope to a name that is not one of the target's own declarations.
func (sc *nameScan) inScope(x *syntax.IdentExpr) bool {
	o := sc.s.scopeName(x)
	return o != nil && !sc.isObject(o)
}

// statesKey reports v, a line literal's key field, stating the target's key: a literal, or a bare
// name no scope resolves, typed `ref C`, with the key's text; a constant, shared, and a selector,
// naming C's entry, are not (API.md E11; DECISIONS 318, 491).
func (sc *nameScan) statesKey(v syntax.Expr) bool {
	x, isIdent := v.(*syntax.IdentExpr)
	if !isIdent {
		match, literal := sc.targetKey(v)
		return match && literal
	}
	_, isRef := baseOf(sc.s.info.Types[x]).(*types.RefType)
	id := sc.tg.ident
	return isRef && sc.s.info.Uses[x] == nil && !id.Key.IsInt && id.Key.S == x.Name
}
