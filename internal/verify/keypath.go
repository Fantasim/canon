package verify

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// KeyTypeAt is the key type P9 quotes by for map m where t is declared: t's, unless it holds an application, computed into m's by verification (log-2026-09-29 M4 U13-r).
func KeyTypeAt(t types.Type, m *value.Map) types.Type {
	if mt, ok := declaredMap(t); ok && !holdsApp(mt.Key) {
		return mt.Key
	}
	return mapKeyType(m)
}

// ListAt is the list type API.md P8 names l's elements by where t is declared: t's, else the
// type l is stored with; et is the element type t declares, nil when t declares no list. The
// walk, rejudge and rules decide "keyed" by it alike (log-2026-09-29 M4 U13-r2).
func ListAt(t types.Type, l *value.List) (lt *types.ListType, et types.Type) {
	if lt, ok := Declared(t).(*types.ListType); ok {
		return lt, lt.Elem
	}
	lt, _ = Declared(l.T).(*types.ListType)
	return lt, nil
}

// declaredMap is the map type t declares, if any.
func declaredMap(t types.Type) (*types.MapType, bool) {
	mt, ok := Declared(t).(*types.MapType)
	return mt, ok
}

// Declared is the type t declares under its aliases, refinements and optional; nil for nil.
func Declared(t types.Type) types.Type {
	for t != nil {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.Refined:
			t = x.Of
		case *types.OptionalType:
			t = x.Elem
		default:
			return t
		}
	}
	return nil
}

// keyPath is the path of key k of a map whose key type kt is declared in e, a literal of kt as
// computed there quoted (API.md P9, log-2026-09-29 M4 U13-r).
func (w *walker) keyPath(at *Path, k value.Value, kt types.Type, e *env) *Path {
	if s, ok := k.(*value.Str); ok && w.literalIn(s.V, kt, e) {
		return at.quoted(s.V)
	}
	return at.MapKey(k, nil)
}

// literalIn reports s a literal of t computed in e, meeting in order the applications walking the key meets (TYPES.md §11.6).
func (w *walker) literalIn(s string, t types.Type, e *env) bool {
	for t != nil && !w.stopped() {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.Refined:
			t = x.Of
		case *types.OptionalType:
			t = x.Elem
		case *types.LitUnionType:
			if slices.Contains(x.Literals, s) {
				return true
			}
			t = x.Of
		case *types.TypeAppType:
			bt, inner, ok := w.branch(x, e)
			if !ok {
				return false
			}
			t, e = bt, inner
		default:
			return false
		}
	}
	return false
}
