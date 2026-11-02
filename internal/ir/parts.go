package ir

import "strconv"

// PartEnds cuts a function's body of whole units, lines[i] the lines of unit i, into parts whose braces hold at most limit lines (CODEGEN.md §2.11, DECISIONS 435; generators pass PartLines, tests a smaller seam). whole is the lines the uncut body holds outside its units, part the lines each part adds inside its own braces (a Decode<N>'s `return true;`). It is greedy and in order: a unit alone over limit-part is a part of its own, and a zero-line unit never starts one. It returns the exclusive end index of each part, the last one len(lines), and nil when no cut is needed (units+whole fit limit) or none helps (one part still over limit). The one cut function gen/cpp and gen/go both use.
func PartEnds(lines []int, limit, whole, part int) []int {
	total := 0
	for _, n := range lines {
		total += n
	}
	if total+whole <= limit {
		return nil
	}
	var ends []int
	room, held := limit-part, 0
	for i, n := range lines {
		if n > 0 && held > 0 && held+n > room {
			ends = append(ends, i)
			held = 0
		}
		held += n
	}
	if len(ends) == 0 && held > room {
		return nil
	}
	return append(ends, len(lines))
}

// CppDecodePart is <P>Access's n-th part of a class's detail::Decode, Decode<N> (CODEGEN.md §2.11, §3.5).
func CppDecodePart(n int) string { return CppDecode + strconv.Itoa(n) }

// GoPart is the n-th part of the Go function fn, <fn>_<N>: build<Last>_<N> or decode<T>_<N> (CODEGEN.md §2.11, §3.5). The interior _ is one lowerCamel never writes for a derived name.
func GoPart(fn string, n int) string { return fn + underscore + strconv.Itoa(n) }
