package cli

import (
	"fmt"
	"slices"
	"strings"
)

// edit is one line of a diff: its mark (keepMark, dropMark or addMark) and its text with its line break.
type edit struct {
	mark rune
	text string
}

// hunk is the edits [from, to) of one hunk of a diff.
type hunk struct{ from, to int }

// counts is how many lines of the old text and of the new one there are before an edit.
type counts struct{ old, updated int }

// unifiedDiff is the changes from old to updated as a unified diff of the file name; empty when
// the two are equal.
func unifiedDiff(name string, old, updated []byte) string {
	edits := lineEdits(diffLines(old), diffLines(updated))
	hunks := hunkSpans(edits)
	if len(hunks) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(fmtDiffOld, name) + lineBreak)
	sb.WriteString(fmt.Sprintf(fmtDiffNew, name) + lineBreak)
	before := linesBefore(edits)
	for _, h := range hunks {
		writeHunk(&sb, edits[h.from:h.to], before[h.from])
	}
	return sb.String()
}

// diffLines splits data into lines, each with its line break; the last has none when data has none.
func diffLines(data []byte) []string {
	lines := strings.SplitAfter(string(data), lineBreak)
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// lineEdits is the edit script from a to b: the common head and tail kept, the middle by longest
// common subsequence, or replaced whole when the table would pass maxDiffCells.
func lineEdits(a, b []string) []edit {
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	out := marked(keepMark, a[:head])
	out = append(out, middleEdits(a[head:len(a)-tail], b[head:len(b)-tail])...)
	return append(out, marked(keepMark, a[len(a)-tail:])...)
}

func marked(mark rune, lines []string) []edit {
	out := make([]edit, len(lines))
	for i, l := range lines {
		out[i] = edit{mark: mark, text: l}
	}
	return out
}

// middleEdits diffs two runs with no common head or tail.
func middleEdits(a, b []string) []edit {
	if len(a) == 0 || len(b) == 0 || (len(a)+1)*(len(b)+1) > maxDiffCells {
		return append(marked(dropMark, a), marked(addMark, b)...)
	}
	common := commonTable(a, b)
	width := len(b) + 1
	var out []edit
	for i, j := 0, 0; i < len(a) || j < len(b); {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			out = append(out, edit{keepMark, a[i]})
			i, j = i+1, j+1
		case j == len(b) || i < len(a) && common[(i+1)*width+j] >= common[i*width+j+1]:
			out = append(out, edit{dropMark, a[i]})
			i++
		default:
			out = append(out, edit{addMark, b[j]})
			j++
		}
	}
	return out
}

// commonTable is the length of the longest common subsequence of a[i:] and b[j:] at [i*(len(b)+1)+j].
func commonTable(a, b []string) []int32 {
	width := len(b) + 1
	common := make([]int32, (len(a)+1)*width)
	for i := range slices.Backward(a) {
		for j := range slices.Backward(b) {
			if a[i] == b[j] {
				common[i*width+j] = common[(i+1)*width+j+1] + 1
			} else {
				common[i*width+j] = max(common[(i+1)*width+j], common[i*width+j+1])
			}
		}
	}
	return common
}

// hunkSpans groups the changes with diffContext lines of context around each; hunks that touch merge.
func hunkSpans(edits []edit) []hunk {
	var out []hunk
	for i, e := range edits {
		if e.mark == keepMark {
			continue
		}
		from, to := max(i-diffContext, 0), min(i+diffContext+1, len(edits))
		if n := len(out); n > 0 && from <= out[n-1].to {
			out[n-1].to = to
			continue
		}
		out = append(out, hunk{from, to})
	}
	return out
}

// linesBefore is, for each edit and past the last, how many lines of the old and of the new text precede it.
func linesBefore(edits []edit) []counts {
	out := make([]counts, len(edits)+1)
	for i, e := range edits {
		out[i+1] = out[i]
		if e.mark != addMark {
			out[i+1].old++
		}
		if e.mark != dropMark {
			out[i+1].updated++
		}
	}
	return out
}

// writeHunk writes a hunk's header and lines; start is the line counts before it.
func writeHunk(sb *strings.Builder, edits []edit, start counts) {
	var size counts
	for _, e := range edits {
		if e.mark != addMark {
			size.old++
		}
		if e.mark != dropMark {
			size.updated++
		}
	}
	sb.WriteString(fmt.Sprintf(fmtHunkHead, rangeText(start.old, size.old), rangeText(start.updated, size.updated)) + lineBreak)
	for _, e := range edits {
		sb.WriteRune(e.mark)
		sb.WriteString(e.text)
		if !strings.HasSuffix(e.text, lineBreak) {
			sb.WriteString(lineBreak + noNewlineMark + lineBreak)
		}
	}
}

// rangeText is a hunk range: its first line (the one before, for an empty range) and its count, the count left out when 1.
func rangeText(before, count int) string {
	switch count {
	case 1:
		return fmt.Sprintf(fmtRangeOne, before+1)
	case 0:
		return fmt.Sprintf(fmtRangeMany, before, count)
	}
	return fmt.Sprintf(fmtRangeMany, before+1, count)
}
