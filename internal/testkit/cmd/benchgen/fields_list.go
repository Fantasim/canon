package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// canonList and jsonList wrap a list's already-rendered elements the two ways a list literal
// and a JSON array are written; kept as the one place each bracket appears (magic-string).
func canonList(elems []string) string { return "[" + strings.Join(elems, canonListSep) + "]" }
func jsonList(elems []string) string  { return "[" + strings.Join(elems, jsonListSep) + "]" }

// stringListField is a `[String](..=max)` field, its elements drawn from a fixed word pool.
func stringListField(name, doc string, max int, pool []string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: fmt.Sprintf("[String](..=%d)", max),
		gen: func(r *progen.Rand, _ *pools) rendered {
			n := r.Intn(max + 1)
			elems := make([]string, n)
			for i := range elems {
				elems[i] = strconv.Quote(progen.Pick(r, pool))
			}
			text := canonList(elems)
			return rendered{text, jsonList(elems)}
		},
	}
}

// intListField is a `[Int](..=max)` field, elements in `0..=bound`.
func intListField(name, doc string, max, bound int) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: fmt.Sprintf("[Int](..=%d)", max),
		gen: func(r *progen.Rand, _ *pools) rendered {
			n := r.Intn(max + 1)
			elems := make([]string, n)
			for i := range elems {
				elems[i] = strconv.Itoa(r.Intn(bound + 1))
			}
			text := canonList(elems)
			return rendered{text, text}
		},
	}
}

// statModElem renders one `{ attribute: <ref>, value: <int> }` element, both wires.
func statModElem(r *progen.Rand, categories []string) (canon, json string) {
	attr := progen.Pick(r, categories)
	value := r.Intn(combatStatBound+combatStatBound+1) - combatStatBound
	canon = fmt.Sprintf("{ attribute: %s, value: %d }", attr, value)
	json = fmt.Sprintf(`{"attribute":%s,"value":%d}`, strconv.Quote(attr), value)
	return canon, json
}

// statModListField is `mods`, a `[StatMod](..=max)` field.
func statModListField(name, doc string, max int) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: fmt.Sprintf("[StatMod](..=%d)", max),
		gen: func(r *progen.Rand, p *pools) rendered {
			if len(p.categories) == 0 {
				return rendered{canonList(nil), jsonList(nil)}
			}
			n := r.Intn(max + 1)
			canonElems := make([]string, n)
			jsonElems := make([]string, n)
			for i := range canonElems {
				canonElems[i], jsonElems[i] = statModElem(r, p.categories)
			}
			return rendered{canonList(canonElems), jsonList(jsonElems)}
		},
	}
}
