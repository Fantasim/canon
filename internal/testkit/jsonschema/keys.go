package jsonschema

import (
	"maps"
	"slices"
)

// sortedKeys returns m's keys in byte order (DOCTRINE.md §5).
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
