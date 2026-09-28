package main

import (
	"fmt"
	"strconv"
	"strings"
)

// twinCanon is twin/twin.canon: the same shape of data, read by load.dir instead of entries.
func twinCanon() []byte {
	return []byte(fmt.Sprintf(`/// The JSON twin of items/item.canon's table.
package twin

import items { Item }

let itemsTwin: [Item] keyed by %s = load.dir("data/*.json")
`, fieldItemCode))
}

// itemJSON is one twin/data/<code>.json file: the same value the entry file writes.
func itemJSON(it *itemData) []byte {
	lines := make([]string, 0, len(it.scalars)+1+len(it.caseFields))
	for _, f := range it.scalars {
		lines = append(lines, jsonLine(f.name, f.json))
	}
	lines = append(lines, jsonLine("kind", strconv.Quote(it.caseName)))
	for _, f := range it.caseFields {
		lines = append(lines, jsonLine(f.name, f.json))
	}
	return []byte("{\n" + strings.Join(lines, ",\n") + "\n}\n")
}

// jsonLine is one `"key": value` line of an object built by hand, never a map (DOCTRINE §5).
func jsonLine(key, value string) string {
	return fmt.Sprintf("  %s: %s", strconv.Quote(key), value)
}
