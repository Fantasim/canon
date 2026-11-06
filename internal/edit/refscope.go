package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
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

// keyFieldToken is the token of v, a line literal's key field, stating key k, which Refs lists and
// Rename rewrites alike: a plain string or integer literal of k, a bare word no scope resolves, or a
// member's name, bare or qualified; nil for any other form or key (API.md E11; DECISIONS 491, 494, 499).
func (s *Snapshot) keyFieldToken(v syntax.Expr, k value.Key) syntax.Node {
	switch x := v.(type) {
	case *syntax.IntLit:
		if k.IsInt && x.Value.IsInt64() && x.Value.Int64() == k.I {
			return x
		}
	case *syntax.IdentExpr:
		if !k.IsInt && x.Name == k.S && s.scopeName(x) == nil {
			return x
		}
	case *syntax.SelectorExpr:
		if tok, name, ok := s.memberToken(x); ok && !k.IsInt && name == k.S {
			return tok
		}
	default:
		if text, ok := constString(v); ok && !k.IsInt && text == k.S {
			return v
		}
	}
	return nil
}

// memberToken is the member name token of e, a key the checker resolves to a member, bare or
// qualified, and that name; false for any other key (DECISIONS 490).
func (s *Snapshot) memberToken(e syntax.Expr) (syntax.Node, string, bool) {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		return x, x.Name, isMember(s.info.Uses[x])
	case *syntax.SelectorExpr:
		if x.Name != nil && isMember(s.info.NameUses[x.Name]) {
			return x.Name, x.Name.Name, true
		}
	}
	return nil, "", false
}
