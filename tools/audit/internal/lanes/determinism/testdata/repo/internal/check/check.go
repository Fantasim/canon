// Package check is no output package: its map ranges are free, its markers judged.
package check

func Sum(m map[string]int) int {
	t := 0
	for _, v := range m {
		t += v
	}
	//canon:unordered a sum
	for _, v := range m {
		t += v
	}
	//canon:unordered nothing below
	return t
}
