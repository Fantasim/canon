package format_test

import (
	"fmt"
	"strings"
)

// diffContext is how many equal lines a difference report shows around a change.
const diffContext = 2

// lineDiff reports the lines of want and got that differ, from the first difference, with a
// little context: enough to read a failing layout test.
func lineDiff(want, got []byte) string {
	w := strings.Split(string(want), "\n")
	g := strings.Split(string(got), "\n")
	start := 0
	for start < len(w) && start < len(g) && w[start] == g[start] {
		start++
	}
	ew, eg := len(w), len(g)
	for ew > start && eg > start && w[ew-1] == g[eg-1] {
		ew--
		eg--
	}
	from := max(start-diffContext, 0)
	var b strings.Builder
	for i := from; i < start; i++ {
		fmt.Fprintf(&b, "  %d  %s\n", i+1, w[i])
	}
	for i := start; i < ew; i++ {
		fmt.Fprintf(&b, "- %d  %s\n", i+1, w[i])
	}
	for i := start; i < eg; i++ {
		fmt.Fprintf(&b, "+ %d  %s\n", i+1, g[i])
	}
	return b.String()
}
