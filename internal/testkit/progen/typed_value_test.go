package progen_test

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// floatLits pairs a Canon float literal with its decoded value: a fixed, safe pool, since a
// generated Float need not explore every representable value to exercise the type (STDLIB.md).
var floatLits = []struct {
	text string
	val  float64
}{{"0.5", 0.5}, {"1.25", 1.25}, {"2.0", 2.0}, {"10.75", 10.75}, {"100.125", 100.125}}

// stringPool is where a generated String's text comes from: plain ASCII words, never a Canon or
// Go keyword, never containing a quote or a brace.
var stringPool = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}

// mapKeys is where a generated Map's keys come from, in insertion order; at most len(mapKeys)
// entries, so no run needs two keys alike.
var mapKeys = []string{"a", "b", "c", "d"}

// typeText is ft's Canon type: a leaf, or a leaf wrapped optional, list or map (TYPES.md §2, §6).
func typeText(mo *typedModel, ft fieldType) string {
	base := leafTypeText(mo, ft)
	switch ft.wrap {
	case wrapOptional:
		return base + "?"
	case wrapList:
		return "[" + base + "]"
	case wrapMap:
		return "{String: " + base + "}"
	default:
		return base
	}
}

func leafTypeText(mo *typedModel, ft fieldType) string {
	switch ft.kind {
	case tyString:
		return "String"
	case tyBool:
		return "Bool"
	case tyFloat:
		return "Float"
	case tyRange:
		return fmt.Sprintf("Int(%d..=%d)", ft.lo, ft.hi)
	case tyEnum:
		return mo.enums[ft.ref].name
	default:
		return "Int"
	}
}

// genLeafValue is a literal of ft's unwrapped kind and the value `encoding/json` decodes an emitted json file's "value" to.
func genLeafValue(r *progen.Rand, mo *typedModel, ft fieldType) (string, any) {
	switch ft.kind {
	case tyString:
		s := progen.Pick(r, stringPool)
		return strconv.Quote(s), s
	case tyBool:
		if r.OneIn(2) {
			return "true", true
		}
		return "false", false
	case tyFloat:
		f := progen.Pick(r, floatLits)
		return f.text, f.val
	case tyRange:
		n := rangeValue(r, ft)
		return strconv.FormatInt(n, decimalBase), float64(n)
	case tyEnum:
		m := progen.Pick(r, mo.enums[ft.ref].members)
		return m, m
	default:
		n := r.Intn(1000)
		return strconv.Itoa(n), float64(n)
	}
}

// rangeValue is a value inside ft's Int(lo..=hi) bound.
func rangeValue(r *progen.Rand, ft fieldType) int64 {
	return ft.lo + int64(r.Intn(int(ft.hi-ft.lo+1)))
}

// genValue is ft's literal (wrapped or not) and its json.Unmarshal-decoded expectation.
func genValue(r *progen.Rand, mo *typedModel, ft fieldType) (string, any) {
	switch ft.wrap {
	case wrapOptional:
		if r.OneIn(3) {
			return "none", nil
		}
		return genLeafValue(r, mo, ft)
	case wrapList:
		return genListValue(r, mo, ft)
	case wrapMap:
		return genMapValue(r, mo, ft)
	default:
		return genLeafValue(r, mo, ft)
	}
}

func genListValue(r *progen.Rand, mo *typedModel, ft fieldType) (string, any) {
	n := 1 + r.Intn(3)
	text, vals := "[", make([]any, n)
	for i := range n {
		if i > 0 {
			text += ", "
		}
		lit, v := genLeafValue(r, mo, ft)
		text, vals[i] = text+lit, v
	}
	return text + "]", vals
}

func genMapValue(r *progen.Rand, mo *typedModel, ft fieldType) (string, any) {
	n := min(1+r.Intn(3), len(mapKeys))
	text, vals := "{ ", map[string]any{}
	for i, k := range mapKeys[:n] {
		if i > 0 {
			text += ", "
		}
		lit, v := genLeafValue(r, mo, ft)
		text, vals[k] = text+strconv.Quote(k)+": "+lit, v
	}
	return text + " }", vals
}

// genRecordValue is a record literal and its values, a default omitted always under omit (TYPES.md §15).
func genRecordValue(r *progen.Rand, mo *typedModel, rec recordDef, omit bool) (string, map[string]any) {
	text, vals, first := "{ ", map[string]any{}, true
	for _, f := range rec.fields {
		if f.def != nil && (omit || r.OneIn(2)) {
			vals[f.name] = f.def.val
			continue
		}
		if !first {
			text += ", "
		}
		lit, v := genValue(r, mo, f.ft)
		text += f.name + ": " + lit
		vals[f.name], first = v, false
	}
	return text + " }", vals
}

// canDefault tells a field type this generator can write a default expression for: an unwrapped
// scalar, range or enum (never Float, to keep a default's arithmetic bit-exact, STDLIB.md).
func canDefault(ft fieldType) bool {
	return ft.wrap == wrapNone && ft.kind != tyFloat
}

// genDefault is a field default (TYPES.md §15), added to a base const so TYPES.md §5.1 can type it.
func genDefault(r *progen.Rand, mo *typedModel, ft fieldType) *fieldDefault {
	switch ft.kind {
	case tyBool:
		v := r.OneIn(2)
		return &fieldDefault{expr: strconv.FormatBool(v), val: v}
	case tyEnum:
		m := progen.Pick(r, mo.enums[ft.ref].members)
		return &fieldDefault{expr: m, val: m}
	case tyString:
		s := progen.Pick(r, stringPool)
		return &fieldDefault{expr: zzStrBase + " + " + strconv.Quote(s), val: s}
	case tyRange:
		n := rangeValue(r, ft)
		return &fieldDefault{expr: fmt.Sprintf("%s + %d", zzIntBase, n), val: float64(n)}
	default: // tyInt
		n := int64(r.Intn(1000))
		return &fieldDefault{expr: fmt.Sprintf("%s + %d", zzIntBase, n), val: float64(n)}
	}
}
