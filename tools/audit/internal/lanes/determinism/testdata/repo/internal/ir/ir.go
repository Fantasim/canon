// Package ir holds every form the rule judges.
package ir

import (
	"maps"
	"slices"
)

type byName map[string]int

func Ranges(m map[string]int, n byName, s []int) int {
	t := 0
	for k := range m {
		t += len(k)
	}
	for _, v := range n {
		t += v
	}
	for k := range maps.Keys(m) {
		t += len(k)
	}
	for _, v := range maps.All(m) {
		t += v
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		t += len(k)
	}
	for range s {
		t++
	}
	//canon:unordered a sum does not depend on the order
	for _, v := range m {
		t += v
	}
	for _, v := range m { //canon:unordered a sum again
		t += v
	}
	//canon:unordered
	for _, v := range n {
		t += v
	}
	//canon:unordered a slice is ordered already
	for range s {
		t++
	}
	// canon:unordered is not the marker
	for range m {
		t++
	}
	return t
}
