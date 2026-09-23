package api

import "sort"

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func indexOf(xs []int, x int) int {
	for i, v := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

func hasRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

func hasValue(m map[string]int, x int) bool {
	for _, v := range m {
		if v == x {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxOf(a, b float64) float64 {
	if a < b {
		return b
	} else {
		return a
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func keysOf(m map[int]bool) []int {
	var out []int
	for k := range m {
		out = append(out, k)
	}
	return out
}

func drain(ch chan int) []int {
	var out []int
	for v := range ch {
		out = append(out, v)
	}
	return out
}

var _ = []any{containsString, indexOf, hasRune, hasValue, minInt, maxOf, sortedKeys, keysOf, drain}
