package main

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/testkit/progen"
	"github.com/fantasim/canonlang/internal/types"
)

// formatFloat renders v with the field's fixed decimal precision, the same text on both wires.
func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', floatPrecision, 64) }

// floatField is a `Float(min..=max)` field, drawn at a fixed resolution then rounded.
func floatField(name, doc string, min, max float64) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: fmt.Sprintf("Float(%s..=%s)", formatFloat(min), formatFloat(max)),
		gen: func(r *progen.Rand, _ *pools) rendered {
			frac := float64(r.Intn(floatSteps+1)) / float64(floatSteps)
			text := formatFloat(min + (max-min)*frac)
			return rendered{text, text}
		},
	}
}

// intPairFields is two `Int(0..=max)` fields, lo always at or under hi (weaponCaseChecks needs
// attackMin <= attackMax); the closures share `last`, drawn in this order for one entry at a time.
func intPairFields(nameLo, docLo, nameHi, docHi string, max int) (lo, hi fieldSpec) {
	typeText := fmt.Sprintf("Int(0..=%d)", max)
	var last int
	lo = fieldSpec{name: nameLo, doc: docLo, typeText: typeText, gen: func(r *progen.Rand, _ *pools) rendered {
		last = r.Intn(max + 1)
		text := strconv.Itoa(last)
		return rendered{text, text}
	}}
	hi = fieldSpec{name: nameHi, doc: docHi, typeText: typeText, gen: func(r *progen.Rand, _ *pools) rendered {
		text := strconv.Itoa(last + r.Intn(max-last+1))
		return rendered{text, text}
	}}
	return lo, hi
}

// boolField is a `Bool` field; the literal is the same word on both wires.
func boolField(name, doc string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: types.BoolType.String(),
		gen: func(r *progen.Rand, _ *pools) rendered {
			text := strconv.FormatBool(r.Intn(boolChoices) == 0)
			return rendered{text, text}
		},
	}
}

// durationField is a `Duration(0ms..=maxMs ms)` field; the wire form is the bare ms count.
func durationField(name, doc string, maxMs int) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: fmt.Sprintf("Duration(0ms..=%dms)", maxMs),
		gen: func(r *progen.Rand, _ *pools) rendered {
			ms := r.Intn(maxMs + 1)
			return rendered{fmt.Sprintf("%dms", ms), strconv.Itoa(ms)}
		},
	}
}
