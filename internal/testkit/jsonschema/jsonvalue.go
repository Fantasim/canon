package jsonschema

import (
	"encoding/json"
	"math/big"
)

// jsonKind reports the JSON Schema kind of a decodeJSON value ("integer" is a type value, not a
// kind: satisfiesType checks it separately).
func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return kindNull
	case bool:
		return kindBoolean
	case json.Number:
		return kindNumber
	case string:
		return kindString
	case []any:
		return kindArray
	case map[string]any:
		return kindObject
	}
	return ""
}

// toRat parses a JSON number token exactly; JSON's number grammar is a subset of the syntax
// big.Rat.SetString accepts (decimal mantissa, optional "e" exponent).
func toRat(n json.Number) (*big.Rat, bool) {
	return new(big.Rat).SetString(string(n))
}

// satisfiesType reports whether v has the JSON Schema type named want.
func satisfiesType(v any, want string) bool {
	if want == kindInteger {
		if jsonKind(v) != kindNumber {
			return false
		}
		r, ok := toRat(v.(json.Number))
		return ok && r.IsInt()
	}
	return jsonKind(v) == want
}

// equalJSON is 2020-12's JSON equality: exact numbers, ordered arrays, unordered object keys.
func equalJSON(a, b any) bool {
	ka, kb := jsonKind(a), jsonKind(b)
	if ka != kb {
		return false
	}
	switch ka {
	case kindNull:
		return true
	case kindBoolean:
		return a.(bool) == b.(bool)
	case kindNumber:
		ra, oka := toRat(a.(json.Number))
		rb, okb := toRat(b.(json.Number))
		return oka && okb && ra.Cmp(rb) == 0
	case kindString:
		return a.(string) == b.(string)
	case kindArray:
		return equalArrays(a.([]any), b.([]any))
	case kindObject:
		return equalObjects(a.(map[string]any), b.(map[string]any))
	}
	return false
}

func equalArrays(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equalJSON(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalObjects(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a { //canon:unordered a comparison, each key checked alone
		bv, ok := b[k]
		if !ok || !equalJSON(av, bv) {
			return false
		}
	}
	return true
}
