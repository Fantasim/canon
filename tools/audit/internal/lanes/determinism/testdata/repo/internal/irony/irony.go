// Package irony shares a prefix with ir, and is no output package.
package irony

func Keys(m map[string]int) (out []string) {
	for k := range m {
		out = append(out, k)
	}
	return out
}
