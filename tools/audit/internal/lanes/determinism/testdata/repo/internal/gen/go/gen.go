// Package gogen is under an output directory.
package gogen

func Keys(m map[string]int) (out []string) {
	for k := range m {
		out = append(out, k)
	}
	return out
}
