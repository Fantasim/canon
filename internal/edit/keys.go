package edit

import (
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyMatch reports whether a key value is the one a segment names.
type keyMatch func(value.Value) bool

// keyMatcher reads lit as a literal of the key type kt (API.md P1, P2); false: kt cannot be
// written that way (a word for an integer key), which is ErrBadPath.
func keyMatcher(lit KeyLit, kt types.Type) (keyMatch, bool) {
	switch b := kt.Base().(type) {
	case types.Basic:
		return basicKey(lit, b)
	case *types.EnumType:
		return namedKey(lit, memberNames)
	case *types.VariantKindType:
		return namedKey(lit, caseNames)
	case *types.RefType:
		return refKey(lit, b.Target)
	case *types.LitUnionType:
		return unionKey(lit, b)
	}
	return dependentKey(lit), true
}

// dependentKey reads a key whose type verification computes (DepUnion): a member like an enum
// key, any other value by its text.
func dependentKey(lit KeyLit) keyMatch {
	member, _ := namedKey(lit, memberNames)
	want := litText(lit)
	return func(v value.Value) bool {
		if _, ok := v.(*value.Member); ok {
			return member != nil && member(v)
		}
		return keyText(v) == want
	}
}

// memberNames is an enum member's Canon name and wire value.
func memberNames(v value.Value) (name, wire string, ok bool) {
	m, isMember := v.(*value.Member)
	if !isMember {
		return "", "", false
	}
	return m.Enum.Members[m.Index].Name, m.Enum.Members[m.Index].Wire, true
}

// caseNames is a Kind value's case name and wire value.
func caseNames(v value.Value) (name, wire string, ok bool) {
	k, isKind := v.(*value.CaseKind)
	if !isKind {
		return "", "", false
	}
	return k.T.Variant.Cases[k.Index].Name, k.T.Variant.Cases[k.Index].Wire, true
}

// basicKey is an integer key read from digits, a String key from a word or a JSON string.
func basicKey(lit KeyLit, b types.Basic) (keyMatch, bool) {
	switch {
	case b.K == types.Int && lit.Kind == KeyInt:
		return func(v value.Value) bool {
			n, ok := v.(*value.Int)
			return ok && n.V == lit.Int
		}, true
	case b.K == types.String && lit.Kind != KeyInt:
		return func(v value.Value) bool {
			s, ok := v.(*value.Str)
			return ok && s.V == lit.Text
		}, true
	}
	return nil, false
}

// namedKey is an enum or Kind key: a word is a Canon name, a JSON string a wire value (P1, P2).
func namedKey(lit KeyLit, names func(value.Value) (name, wire string, ok bool)) (keyMatch, bool) {
	if lit.Kind == KeyInt {
		return nil, false
	}
	return func(v value.Value) bool {
		name, wire, ok := names(v)
		if lit.Kind == KeyWord {
			return ok && name == lit.Text
		}
		return ok && wire == lit.Text
	}, true
}

// refKey is a ref key: lit read as a key of the target collection (P1).
func refKey(lit KeyLit, coll *types.Collection) (keyMatch, bool) {
	want, ok := refWant(lit, coll)
	return func(v value.Value) bool {
		r, isRef := v.(*value.Ref)
		return isRef && r.Key == want
	}, ok
}

// refWant is the key lit names in coll: a table's is a String, a keyed list's its key field's.
func refWant(lit KeyLit, coll *types.Collection) (value.Key, bool) {
	var kt types.Type = types.StringType
	if coll != nil && coll.KeyedBy != nil {
		kt = coll.KeyedBy.Type
	}
	switch b := kt.Base().(type) {
	case types.Basic:
		if b.K == types.Int {
			return value.Key{I: lit.Int, IsInt: true}, lit.Kind == KeyInt
		}
		return value.Key{S: lit.Text}, lit.Kind != KeyInt
	case *types.EnumType:
		return memberKey(lit, b)
	case *types.RefType:
		return refWant(lit, b.Target)
	}
	return value.Key{S: litText(lit)}, true
}

// memberKey is the key of an enum-keyed entry: the member's Canon name.
func memberKey(lit KeyLit, e *types.EnumType) (value.Key, bool) {
	if lit.Kind != KeyString {
		return value.Key{S: lit.Text}, lit.Kind == KeyWord
	}
	for _, m := range e.Members {
		if m.Wire == lit.Text {
			return value.Key{S: m.Name}, true
		}
	}
	return value.Key{S: lit.Text}, true
}

// unionKey reads a JSON string equal to a literal as that literal, anything else as the base (P2, TYP-09).
func unionKey(lit KeyLit, u *types.LitUnionType) (keyMatch, bool) {
	for _, l := range u.Literals {
		if lit.Kind == KeyString && l == lit.Text {
			return func(v value.Value) bool {
				s, ok := v.(*value.Str)
				return ok && s.V == l
			}, true
		}
	}
	return keyMatcher(lit, u.Of)
}

// litText is a key literal's text: an integer in decimal, a word or a string as decoded.
func litText(lit KeyLit) string {
	if lit.Kind == KeyInt {
		return strconv.FormatInt(lit.Int, decimalBase)
	}
	return lit.Text
}

// keyText is a key value's text: the name of a member, case or symbol, the text of a string or ref.
func keyText(v value.Value) string {
	switch x := v.(type) {
	case *value.Str:
		return x.V
	case *value.Ref:
		return x.Key.Text()
	case *value.Symbol:
		return x.Name
	}
	return v.CanonText()
}

// keySeg is the canonical segment of a key of type kt (API.md P8, P9): a literal of a literal
// union is always a JSON string.
func keySeg(v value.Value, kt types.Type) Seg {
	switch x := v.(type) {
	case *value.Int:
		return intSeg(x.V)
	case *value.Ref:
		if x.Key.IsInt {
			return intSeg(x.Key.I)
		}
	case *value.Str:
		if u, ok := baseOf(kt).(*types.LitUnionType); ok && slices.Contains(u.Literals, x.V) {
			return Seg{Kind: SegKey, Key: quotedKey(x.V)}
		}
	}
	return Seg{Kind: SegKey, Key: textKey(keyText(v))}
}

// baseOf is t.Base(), nil for no type.
func baseOf(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return t.Base()
}

// entrySeg is a table entry's canonical segment: `.key`, or `["key"]` for a key that is no word (P8, P9).
func entrySeg(k value.Key) Seg {
	switch {
	case k.IsInt:
		return intSeg(k.I)
	case isWord(k.S):
		return Seg{Kind: SegField, Name: k.S}
	}
	return Seg{Kind: SegKey, Key: textKey(k.S)}
}

// intSeg is `[n]`.
func intSeg(n int64) Seg {
	return Seg{Kind: SegKey, Key: KeyLit{Kind: KeyInt, Int: n, Raw: strconv.FormatInt(n, decimalBase)}}
}

// textKey is s as a word when it is one, else as a JSON string escaped as WIRE.md does (P9).
func textKey(s string) KeyLit {
	if isWord(s) {
		return KeyLit{Kind: KeyWord, Text: s, Raw: s}
	}
	return quotedKey(s)
}

// quotedKey is s as a JSON string escaped as WIRE.md does.
func quotedKey(s string) KeyLit {
	return KeyLit{Kind: KeyString, Text: s, Raw: string(diag.AppendJSONString(nil, s))}
}

// isWord reports s is exactly one word of API.md §6.1.
func isWord(s string) bool {
	return s != "" && wordLen(s) == len(s)
}
