package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// projectCanon is the standalone project's root file; `icons` roots the asset field.
func projectCanon() []byte {
	return []byte(`/// The NFR-01 benchmark project (IMPLEMENTATION-PLAN.md §7.6), written by
/// internal/testkit/cmd/benchgen. Not committed: regenerate it, never edit it by hand.
project bench {
  canon: "0.1"

  roots {
    icons: "items/icons"
  }
}
`)
}

// monsterCanon is the monster table's package: item.canon's dropsFrom refs into it.
func monsterCanon(r *progen.Rand, m *genModel) []byte {
	var b strings.Builder
	b.WriteString("/// Monsters an item may drop from.\n")
	b.WriteString("package monster\n\n")
	b.WriteString("/// A monster an item may drop from.\n")
	b.WriteString("record Monster {\n")
	b.WriteString("  /// Display name shown to players.\n  name: String(1..)\n")
	fmt.Fprintf(&b, "  /// The monster's level.\n  level: Int(1..=%d)\n", levelBound)
	fmt.Fprintf(&b, "  /// Hit points at full health.\n  hp: Int(1..=%d)\n}\n\n", hpBound)
	b.WriteString("let monsters: table Monster = {\n")
	for _, key := range m.monsters {
		name := progen.Pick(r, nouns())
		level := 1 + r.Intn(levelBound)
		hp := 1 + r.Intn(hpBound)
		fmt.Fprintf(&b, "  %s { name: %s, level: %d, hp: %d }\n", key, strconv.Quote(name), level, hp)
	}
	b.WriteString("}\n")
	return []byte(b.String())
}
