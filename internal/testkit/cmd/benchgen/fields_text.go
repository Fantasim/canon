package main

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// stringOptField is an optional plain-text field, drawn from a fixed phrase pool.
func stringOptField(name, doc string, pool []string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: "String?",
		gen: func(r *progen.Rand, _ *pools) rendered {
			if !present(r) {
				return noneRendered()
			}
			q := strconv.Quote(progen.Pick(r, pool))
			return rendered{q, q}
		},
	}
}

// regexOptField is `iconTint`, an optional `#RRGGBB` string refinement, not an asset.
func regexOptField(name, doc string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: `String(/^#[0-9A-Fa-f]{6}$/)?`,
		gen: func(r *progen.Rand, _ *pools) rendered {
			if !present(r) {
				return noneRendered()
			}
			var b strings.Builder
			b.WriteByte('#')
			digits := hexDigits()
			for range hexColorDigits {
				b.WriteByte(digits[r.Intn(hexBase)])
			}
			q := strconv.Quote(b.String())
			return rendered{q, q}
		},
	}
}
