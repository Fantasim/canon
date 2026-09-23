package value

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// Equal is value equality (TYP-08): identities when both sides carry one, else structure.
// The caller has converted both sides first: a ref is never dereferenced here.
func Equal(a, b Value) bool {
	if ia, ib := identity(a), identity(b); ia != nil && ib != nil {
		return ia.same(ib)
	}
	if e, ok := a.(equaler); ok {
		return e.equal(b)
	}
	return a == b
}

// equaler is the structural equality of each value type of this package.
type equaler interface {
	equal(o Value) bool
}

// identity is the identity a value carries: a ref's target entry, or an entry's own.
func identity(v Value) *Identity {
	switch x := v.(type) {
	case *Ref:
		return x.identity()
	case *Record:
		return x.Ident
	}
	return nil
}

func equalSlices(a, b []Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// nested is the text form of a value inside a composite: strings are quoted (STD-06).
func nested(v Value) string {
	if s, ok := v.(*Str); ok {
		return types.QuoteString(s.V)
	}
	return v.CanonText()
}

func nestedList(vs []Value) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = nested(v)
	}
	return strings.Join(parts, textSep)
}
