package cli

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(hunkHeaderPattern)

// fixHunkStarts sets each new start to the old start plus the earlier hunks' net lines. CLI.md §3.6
func fixHunkStarts(diff string) string {
	lines := strings.SplitAfter(diff, lineBreak)
	delta := 0
	for i, l := range lines {
		if !strings.HasPrefix(l, hunkMarker) {
			continue
		}
		m := hunkHeader.FindStringSubmatch(strings.TrimSuffix(l, lineBreak))
		if len(m) != hunkHeaderGroups {
			continue
		}
		oldStart, oldCount := atoi(m[grpOldStart]), countOf(m[grpOldCount])
		newCount := countOf(m[grpNewCount])
		newStart := atoi(m[grpNewStart])
		if oldStart > 0 && newStart > 0 {
			newStart = oldStart + delta
		}
		delta += newCount - oldCount
		lines[i] = hunkHeaderLine(m, newStart) + lineBreak
	}
	return strings.Join(lines, "")
}

// hunkHeaderLine is the header of match m with newStart as the new-side start, the counts as written.
func hunkHeaderLine(m []string, newStart int) string {
	newSide := strconv.Itoa(newStart)
	if m[grpNewCount] != "" {
		newSide += "," + m[grpNewCount]
	}
	oldSide := m[grpOldStart]
	if m[grpOldCount] != "" {
		oldSide += "," + m[grpOldCount]
	}
	return "@@ -" + oldSide + " +" + newSide + " @@" + m[grpTail]
}

// countOf is a header range count, one when the header omits it.
func countOf(s string) int {
	if s == "" {
		return hunkDefaultCount
	}
	return atoi(s)
}

func atoi(s string) int {
	n, _ := strconv.ParseInt(s, decimalBase, strconv.IntSize)
	return int(n)
}
