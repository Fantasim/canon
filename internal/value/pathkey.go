package value

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
)

// PathText is the text API.md P9 writes k with in a value path: an integer in decimal, a word
// bare, any other key as a JSON string.
func (k Key) PathText() string {
	if k.IsInt || IsWord(k.S) {
		return k.Text()
	}
	return string(diag.AppendJSONString(nil, k.S))
}

// IsWord reports a word of SPEC §2.4; "_" alone is not one.
func IsWord(s string) bool {
	for i, c := range s {
		letter := c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return s != "" && s != underscore
}

// mapKeyOf is the key a map key value is written with in a path (API.md P9): an integer's
// number, a ref's target key, any other key's canonical text.
func mapKeyOf(v Value) Key {
	switch k := v.(type) {
	case *Int:
		return Key{I: k.V, IsInt: true}
	case *Ref:
		return k.Key
	}
	return Key{S: v.CanonText()}
}

// PathKey is the text API.md P9 writes the map key k with, kt the key type declared where the
// map is (log-2026-09-29 "P9 quoting rule"): a literal of a literal-union key type is a JSON
// string, since P2 reads a word as the union's base; any other key is mapKeyOf's PathText.
func PathKey(k Value, kt types.Type) string {
	if s, ok := k.(*Str); ok && keyLiteral(s.V, kt) {
		return string(diag.AppendJSONString(nil, s.V))
	}
	return mapKeyOf(k).PathText()
}

// keyLiteral reports s a literal of kt under its aliases, refinements and match-less expansions (TYPES.md §11.4).
func keyLiteral(s string, kt types.Type) bool {
	for kt != nil {
		switch x := kt.(type) {
		case *types.Alias:
			kt = x.Def
		case *types.Refined:
			kt = x.Of
		case *types.OptionalType:
			kt = x.Elem
		case *types.LitUnionType:
			if slices.Contains(x.Literals, s) {
				return true
			}
			kt = x.Of
		case *types.TypeAppType:
			kt = expansion(x)
		default:
			return false
		}
	}
	return false
}

// expansion is the body of app's type function when it has no `match`, nil otherwise.
func expansion(app *types.TypeAppType) types.Type {
	if app.Fn == nil || app.Fn.Scrutinee != nil {
		return nil
	}
	return app.Fn.Body
}
