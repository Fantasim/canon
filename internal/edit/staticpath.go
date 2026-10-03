package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// StaticPath is the value path e names when it is a let root read by fields, literal indices and
// keys only; false for any other expression (a parameter, a call, a computed index).
func (s *Snapshot) StaticPath(e syntax.Expr) (Path, bool) {
	info := s.info
	switch x := syntax.Unparen(e).(type) {
	case *syntax.IdentExpr:
		o := info.Uses[x]
		return Path{Package: pkgOf(o), Root: x.Name}, o != nil && o.Kind() == check.ObjLet
	case *syntax.SelectorExpr:
		if o := info.NameUses[x.Name]; info.Selections[x] == nil && o != nil && o.Kind() == check.ObjLet {
			return Path{Package: o.Pkg(), Root: o.Name()}, true
		}
		if x.Optional || x.X == nil || info.Selections[x] == nil {
			return Path{}, false
		}
		return s.extended(x.X, Seg{Kind: SegField, Name: x.Name.Name})
	case *syntax.IndexExpr:
		k, ok := s.literalKey(x.Index)
		if !ok {
			return Path{}, false
		}
		return s.extended(x.X, Seg{Kind: SegKey, Key: k})
	}
	return Path{}, false
}

// extended is the static path of e with seg after it.
func (s *Snapshot) extended(e syntax.Expr, seg Seg) (Path, bool) {
	p, ok := s.StaticPath(e)
	p.Segs = append(slices.Clip(p.Segs), seg)
	return p, ok
}

// literalKey is an index written as a literal: a non-negative integer, a string, or a symbolic key.
func (s *Snapshot) literalKey(e syntax.Expr) (KeyLit, bool) {
	switch x := syntax.Unparen(e).(type) {
	case *syntax.IntLit:
		return KeyLit{Kind: KeyInt, Int: x.Value.Int64()}, x.Value.IsInt64() && x.Value.Sign() >= 0
	case *syntax.IdentExpr:
		return KeyLit{Kind: KeyWord, Text: x.Name}, s.info.Keys[x] != nil
	}
	text, ok := constString(e)
	return KeyLit{Kind: KeyString, Text: text}, ok
}

// pkgOf is o's package path, "" for no object.
func pkgOf(o check.Object) string {
	if o == nil {
		return ""
	}
	return o.Pkg()
}
