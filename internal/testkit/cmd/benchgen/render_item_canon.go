package main

import (
	"fmt"
	"strings"
)

// itemCanon is items/item.canon: the package every entry file and the twin both read.
func itemCanon(n int, m *genModel, shared sharedSpecs, caseFields []fieldSpec) []byte {
	var b strings.Builder
	b.WriteString(itemPreamble())
	fmt.Fprintf(&b, "const MAX_ITEMS_PER_CATEGORY = %d\n\n", n)
	writeItemRecord(&b, shared)
	writeItemVariant(&b, m.cases, caseFields)
	writeItemsTable(&b)
	writeItemChecks(&b)
	writeItemViewAndEmit(&b)
	return []byte(b.String())
}

// itemPreamble is everything before the Item record.
func itemPreamble() string {
	return fmt.Sprintf(`/// Items: the NFR-01 benchmark project, written by internal/testkit/cmd/benchgen.
package items

import monster { monsters }

local let categories = load.defines("defines/categories.h")

/// An item's icon: a PNG under the icons root.
type ItemIcon = asset("@icons", ext: [png])

%s%s/// An attribute and how much it changes.
record StatMod {
  /// The attribute modified.
  attribute: ref categories
  /// How much the attribute changes.
  value: Int(-%d..=%d)
}

`, enumDecl(qualityTypeName, "An item's quality tier.", qualityMembers()),
		enumDecl(elementTypeName, "An item's elemental affinity.", elementMembers()),
		combatStatBound, combatStatBound)
}

// enumDecl is a documented `enum Name { A, B, C }`, its members read from the same generator
// schema.go uses.
func enumDecl(name, doc string, members []string) string {
	return fmt.Sprintf("/// %s\nenum %s { %s }\n\n", doc, name, strings.Join(members, canonListSep))
}

// writeItemField writes one field's doc comment, then its name and type.
func writeItemField(b *strings.Builder, f fieldSpec, def string) {
	fmt.Fprintf(b, "  /// %s\n  %s: %s%s\n", f.doc, f.name, f.typeText, def)
}

// writeItemRecord writes the Item record, reusing the shared generator's own type text so the
// declaration can never drift from what generateItem draws.
func writeItemRecord(b *strings.Builder, shared sharedSpecs) {
	b.WriteString("/// An item: 12 shared fields plus a variant of kind-specific fields.\n")
	fmt.Fprintf(b, "record Item {\n  /// Stable identifier; matches the entry's file name.\n  %s: String(/^ITM_[0-9]{6}$/)\n",
		fieldItemCode)
	fmt.Fprintf(b, "  /// Name shown to players.\n  %s: String(1..)\n", fieldDisplayName)
	writeItemField(b, shared.description, "")
	b.WriteString("  /// Thumbnail shown in inventories and pickers.\n  icon: ItemIcon\n")
	writeItemField(b, shared.category, "")
	writeItemField(b, shared.dropsFrom, "")
	writeItemField(b, shared.tier, "")
	writeItemField(b, shared.cost, "")
	writeItemField(b, shared.weight, "")
	writeItemField(b, shared.stackMax, fmt.Sprintf(" = %d", stackDefault))
	writeItemField(b, shared.rarity, "")
	b.WriteString("  /// What the item is; each case carries its own fields.\n  kind: ItemKind @json(inline)\n\n")
	fmt.Fprintf(b, "  check (dropsFrom == none) or tier >= %d\n", minDropTier)
	fmt.Fprintf(b, "    else \"an item that drops from a monster must be at least tier %d\"\n", minDropTier)
	b.WriteString("}\n\n")
}

// writeItemVariant writes the 8-case ItemKind variant; the first case alone carries the two
// checks that need case fields.
func writeItemVariant(b *strings.Builder, cases []string, fields []fieldSpec) {
	b.WriteString("/// What kind of item this is; each case carries its own fields.\n")
	b.WriteString("variant ItemKind {\n")
	for i, c := range cases {
		fmt.Fprintf(b, "  %s {\n", c)
		for _, f := range fields {
			fmt.Fprintf(b, "    /// %s\n    %s: %s\n", f.doc, f.name, f.typeText)
		}
		if i == 0 {
			b.WriteString(weaponCaseChecks())
		}
		b.WriteString("  }\n")
	}
	b.WriteString("}\n\n")
}

// weaponCaseChecks are the variant's own 2 record checks, on the first case only.
func weaponCaseChecks() string {
	return `
    check attackMin <= attackMax else "attackMin must not exceed attackMax"
    warn critChance == 0.0 or critMultiplier > 1.0
      else "a nonzero critChance should carry a critMultiplier above 1.0"
`
}

// writeItemsTable places the stable table, one entry file per item, directly under the package
// directory (API.md N3): items/{kind}/{id}.canon.
func writeItemsTable(b *strings.Builder) {
	b.WriteString("@files(\"{kind}/{id}.canon\")\n")
	b.WriteString("let items: stable table Item = {}\n\n")
}

// writeItemChecks are the 2 package checks: uniqueness, then a group-by.
func writeItemChecks(b *strings.Builder) {
	fmt.Fprintf(b, "check items.values().map(.%s).isUnique() else \"item codes must be unique\"\n\n",
		fieldItemCode)
	b.WriteString(`check {
  for cat, rows in items.values().groupBy(.category) {
    if rows.len() > MAX_ITEMS_PER_CATEGORY {
      fail(rows[0], "category {cat.id} holds {rows.len()} items, more than {MAX_ITEMS_PER_CATEGORY}")
    }
  }
}

`)
}

// writeItemViewAndEmit is the collection's view and what the package emits.
func writeItemViewAndEmit(b *strings.Builder) {
	fmt.Fprintf(b, `view Item {
  title "{%s}"
  preview icon
  search { %s, %s, category }
  filters { kind, category, rarity }
  columns { attackMin, tier 70, cost 100, rarity 100 }
}

emit json { out: "out/items.json", values: [items] }
emit view { out: "out/items.view.json" }
`, fieldDisplayName, fieldItemCode, fieldDisplayName)
}
