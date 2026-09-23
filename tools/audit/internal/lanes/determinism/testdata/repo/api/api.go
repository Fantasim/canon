// Package canon is an output package at the top level.
package canon

func Keys(m map[string]int) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
