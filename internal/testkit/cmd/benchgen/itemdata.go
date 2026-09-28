package main

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// namedValue pairs a field's name with its drawn value, one render per name (no separate list).
type namedValue struct {
	name string
	rendered
}

// itemData is one entry's values: `scalars` are the Item record's shared fields, in order.
type itemData struct {
	code       string
	scalars    []namedValue
	caseName   string
	caseFields []namedValue
}

// sharedSpecs are the 8 shared fields an ordinary fieldSpec expresses; itemCode, displayName
// and icon are entry-specific, built directly in generateItem.
type sharedSpecs struct {
	description, category, dropsFrom, tier fieldSpec
	cost, weight, stackMax, rarity         fieldSpec
}

// buildSharedSpecs is called once per run, not once per entry.
func buildSharedSpecs() sharedSpecs {
	return sharedSpecs{
		description: stringOptField("description", "Longer description shown in the item's detail panel.",
			flavorPhrases()),
		category:  refField("category", "The item's category, from the game's define table.", identCategories),
		dropsFrom: refOptField("dropsFrom", "The monster this item drops from, if any.", identMonsters),
		tier:      intFieldFrom(1, "tier", "Power tier from 1 (common) to 10 (mythic).", tierBound),
		cost:      intField("cost", "Price in the default shop currency.", costBound),
		weight:    floatField("weight", "Inventory weight.", 0, weightFloatBound),
		stackMax:  intFieldFrom(1, "stackMax", "How many fit in one inventory slot.", stackBound),
		rarity:    enumField("rarity", "The item's rarity.", qualityTypeName, qualityMembers()),
	}
}

// generateItem draws entry index's whole value, in declaration order.
func generateItem(r *progen.Rand, index int, m *genModel, shared sharedSpecs, caseFields []fieldSpec) *itemData {
	p := &pools{categories: m.refPool, monsters: m.monsters}
	code := itemCode(index)
	name := fmt.Sprintf("%s %s", progen.Pick(r, adjectives()), progen.Pick(r, nouns()))
	tier, drops := drawTierAndDrops(r, shared.dropsFrom, p)
	it := &itemData{
		code: code,
		scalars: []namedValue{
			{fieldItemCode, quotedBoth(code)},
			{fieldDisplayName, quotedBoth(name)},
			{shared.description.name, shared.description.gen(r, p)},
			{"icon", quotedBoth(code + iconExt)},
			{shared.category.name, shared.category.gen(r, p)},
			{shared.dropsFrom.name, drops},
			{shared.tier.name, tier},
			{shared.cost.name, shared.cost.gen(r, p)},
			{shared.weight.name, shared.weight.gen(r, p)},
			{shared.stackMax.name, shared.stackMax.gen(r, p)},
			{shared.rarity.name, shared.rarity.gen(r, p)},
		},
		caseName:   progen.Pick(r, m.cases),
		caseFields: make([]namedValue, len(caseFields)),
	}
	for i, f := range caseFields {
		it.caseFields[i] = namedValue{name: f.name, rendered: f.gen(r, p)}
	}
	return it
}

// drawTierAndDrops draws tier first and caps dropsFrom by it, so the Item record's own check
// (tier >= minDropTier whenever dropsFrom is set) always holds.
func drawTierAndDrops(r *progen.Rand, dropsFrom fieldSpec, p *pools) (tier, drops rendered) {
	tierVal := 1 + r.Intn(tierBound)
	text := strconv.Itoa(tierVal)
	tier = rendered{text, text}
	drops = noneRendered()
	if tierVal >= minDropTier {
		drops = dropsFrom.gen(r, p)
	}
	return tier, drops
}

// quotedBoth is a plain string field whose value never differs between the two wires.
func quotedBoth(s string) rendered {
	q := strconv.Quote(s)
	return rendered{q, q}
}
