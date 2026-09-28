package main

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// noneRendered is an optional field's absent value: `none` in Canon, `null` in JSON.
func noneRendered() rendered { return rendered{"none", jsonNull} }

// present draws whether an optional field has a value this time.
func present(r *progen.Rand) bool { return !r.OneIn(optionalOneIn) }

// poolFor is target's collection (constants.go: identCategories, identMonsters).
func poolFor(target string, p *pools) []string {
	if target == identMonsters {
		return p.monsters
	}
	return p.categories
}

// enumField is a required enum field: the same member name, bare in Canon, quoted in JSON.
func enumField(name, doc, typeName string, members []string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: typeName,
		gen: func(r *progen.Rand, _ *pools) rendered {
			m := progen.Pick(r, members)
			return rendered{m, strconv.Quote(m)}
		},
	}
}

// enumOptField is an optional enum field.
func enumOptField(name, doc, typeName string, members []string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: typeName + "?",
		gen: func(r *progen.Rand, _ *pools) rendered {
			if !present(r) {
				return noneRendered()
			}
			m := progen.Pick(r, members)
			return rendered{m, strconv.Quote(m)}
		},
	}
}

// refTypeText is `ref target`, `?` appended for an optional one.
func refTypeText(target string, optional bool) string {
	suffix := ""
	if optional {
		suffix = "?"
	}
	return "ref " + target + suffix
}

// refField is a required `ref` field into one of the two pools.
func refField(name, doc, target string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: refTypeText(target, false),
		gen: func(r *progen.Rand, p *pools) rendered {
			id := progen.Pick(r, poolFor(target, p))
			return rendered{id, strconv.Quote(id)}
		},
	}
}

// refOptField is an optional `ref` field into one of the two pools.
func refOptField(name, doc, target string) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: refTypeText(target, true),
		gen: func(r *progen.Rand, p *pools) rendered {
			pool := poolFor(target, p)
			if !present(r) || len(pool) == 0 {
				return noneRendered()
			}
			id := progen.Pick(r, pool)
			return rendered{id, strconv.Quote(id)}
		},
	}
}
