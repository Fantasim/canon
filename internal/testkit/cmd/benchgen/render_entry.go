package main

import (
	"fmt"
	"strings"
)

// entryCanon is one items/<case>/<code>.canon file: the scalar fields, then the case's own.
func entryCanon(it *itemData) []byte {
	var b strings.Builder
	b.WriteString("package items\n\n")
	fmt.Fprintf(&b, "entry items.%s {\n", it.code)
	for _, f := range it.scalars {
		fmt.Fprintf(&b, "  %s: %s\n", f.name, f.canon)
	}
	fmt.Fprintf(&b, "  kind: %s {\n", it.caseName)
	for _, f := range it.caseFields {
		fmt.Fprintf(&b, "    %s: %s\n", f.name, f.canon)
	}
	b.WriteString("  }\n}\n")
	return []byte(b.String())
}

// entryPath is one item's @files-placed path, relative to items/item.canon's own directory.
func entryPath(it *itemData) string {
	return fmt.Sprintf("%s/%s.canon", it.caseName, it.code)
}
