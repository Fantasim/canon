package main

// caseNames are the variant's 8 cases (§7.6), in declaration order.
func caseNames() []string {
	return []string{
		"IK_WEAPON", "IK_ARMOR", "IK_CONSUMABLE", "IK_MATERIAL",
		"IK_QUEST", "IK_MOUNT", "IK_HOUSING", "IK_GEM",
	}
}

// qualityMembers are the members of the `Quality` enum.
func qualityMembers() []string {
	return []string{"COMMON", "UNCOMMON", "RARE", "EPIC", "LEGENDARY"}
}

// elementMembers are the members of the `Element` enum.
func elementMembers() []string {
	return []string{"NEUTRAL", "FIRE", "WATER", "WIND", "EARTH"}
}

// adjectives and nouns compose an item's display name.
func adjectives() []string {
	return []string{
		"Iron", "Steel", "Ancient", "Cursed", "Blessed", "Shadow", "Radiant", "Frozen",
		"Molten", "Silent", "Gilded", "Rusted", "Feral", "Hollow", "Storm",
	}
}

func nouns() []string {
	return []string{
		"Blade", "Axe", "Shield", "Ring", "Amulet", "Cloak", "Bow", "Staff", "Helm",
		"Gauntlet", "Boots", "Charm", "Lantern", "Totem", "Orb",
	}
}

// flavorPhrases and lorePhrases fill the optional text fields.
func flavorPhrases() []string {
	return []string{
		"Still warm from the forge.", "Hums faintly in moonlight.", "Smells of old rain.",
		"Its edge never dulls.", "Once carried by a nameless hero.",
	}
}

func lorePhrases() []string {
	return []string{
		"Recovered from a sunken keep.", "Traded for a single word of truth.",
		"Forged before the first war.", "Its maker left no name.",
	}
}

// keywordPool fills a `keywords` list field.
func keywordPool() []string {
	return []string{
		"ancient", "legendary", "cursed", "blessed", "forged", "enchanted", "rare",
		"common", "mystic", "shiny",
	}
}

// hexDigits are the digits of a generated `iconTint` colour.
func hexDigits() string { return "0123456789ABCDEF" }
