package main

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// rendered is one field's value: the Canon entry literal and its JSON twin.
type rendered struct{ canon, json string }

// pools are the collections a field's generator may pick a ref from.
type pools struct {
	categories []string
	monsters   []string
}

// fieldSpec is one case field: its doc comment, its Canon type text and its value generator.
type fieldSpec struct {
	name     string
	doc      string
	typeText string
	gen      func(r *progen.Rand, p *pools) rendered
}

// caseFieldSpecs is the 40 fields every case of ItemKind shares.
func caseFieldSpecs() []fieldSpec {
	attackMin, attackMax := intPairFields("attackMin", "Lowest base damage.", "attackMax",
		"Highest base damage.", combatStatBound)
	return []fieldSpec{
		attackMin, attackMax,
		intField("defense", "Damage absorbed before health is spent.", combatStatBound),
		intField("magicPower", "Bonus applied to magic damage dealt.", combatStatBound),
		intField("magicResist", "Magic damage absorbed before health is spent.", combatStatBound),
		intField("durability", "Uses left before the item breaks.", durabilityBound),
		intFieldFrom(1, "weightClass", "How heavy the item is to carry.", weightClassBound),
		intFieldFrom(1, "levelReq", "Level required to equip or use the item.", levelBound),
		intField("staminaCost", "Stamina spent per use.", statCostBound),
		intField("manaCost", "Mana spent per use.", statCostBound),
		intField("socketCount", "Empty sockets for gems.", socketBound),
		intField("upgradeLevel", "Times the item has been upgraded.", upgradeBound),
		intField("vendorPrice", "Price a vendor pays to buy it.", vendorPriceBound),
		intField("comboCount", "Hits in the item's combo chain.", comboBound),
		intField("sortWeight", "Tie-breaker for list ordering.", sortBound),

		floatField("critChance", "Chance a hit lands as a critical.", 0, floatUnit),
		floatField("critMultiplier", "Damage multiplier on a critical hit.",
			critMultiplierMin, critMultiplierMax),
		floatField("speedBonus", "Attack speed bonus or penalty.", floatNegOne, floatUnit),
		floatField("rangeDistance", "Maximum effective range.", 0, floatRange50),
		floatField("radius", "Radius of the item's area effect.", 0, floatRadius20),
		floatField("dropRate", "Chance the item drops from its source.", 0, floatUnit),
		floatField("encumbrance", "Inventory weight the item adds.", 0, encumbranceBound),
		floatField("procChance", "Chance an on-hit effect triggers.", 0, floatUnit),

		boolField("isTradeable", "Whether the item can be traded."),
		boolField("isStackable", "Whether several copies share one slot."),
		boolField("isQuestItem", "Whether a quest requires this item."),
		boolField("requiresQuest", "Whether equipping needs a quest completed first."),

		durationField("cooldown", "Wait before the item can be used again.", cooldownMaxMs),
		durationField("castTime", "Time spent casting before the effect fires.", castTimeMaxMs),
		durationField("effectDuration", "How long the item's effect lasts.", effectMaxMs),

		enumField("quality", "The item's quality tier.", qualityTypeName, qualityMembers()),
		enumField("element", "The item's elemental affinity.", elementTypeName, elementMembers()),
		enumOptField("secondaryQuality", "A second quality tier, when the item carries one.",
			qualityTypeName, qualityMembers()),
		refOptField("setBonus", "The set this item belongs to, if any.", identCategories),

		stringOptField("flavorText", "Flavor text shown in the item's tooltip.", flavorPhrases()),
		stringOptField("lore", "Background lore for the item.", lorePhrases()),
		regexOptField("iconTint", "A tint colour applied over the icon."),

		stringListField("keywords", "Search keywords for this item.", keywordsMax, keywordPool()),
		intListField("bonusStats", "Extra stat rolls beyond the base fields.", bonusStatsMax, combatStatBound),
		statModListField("mods", "Attribute modifiers granted while equipped.", modsMax),
	}
}

// intField is an `Int(0..=max)` field, rendered the same way in both texts.
func intField(name, doc string, max int) fieldSpec { return intFieldFrom(0, name, doc, max) }

// intFieldFrom is an `Int(min..=max)` field.
func intFieldFrom(min int, name, doc string, max int) fieldSpec {
	return fieldSpec{
		name: name, doc: doc,
		typeText: fmt.Sprintf("Int(%d..=%d)", min, max),
		gen: func(r *progen.Rand, _ *pools) rendered {
			text := strconv.Itoa(min + r.Intn(max-min+1))
			return rendered{text, text}
		},
	}
}
