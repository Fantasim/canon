package cli

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// CLI.md §3.6: each hunk's new start is its old start plus the net lines the earlier hunks added.
func TestUnifiedDiffHeaders(t *testing.T) {
	numbered := func(n int) []string {
		l := make([]string, n)
		for i := range l {
			l[i] = "line " + strconv.Itoa(i+1)
		}
		return l
	}
	edit := func(at ...int) (string, string) {
		o := numbered(60)
		n := numbered(60)
		for _, i := range at {
			n[i] = "changed " + strconv.Itoa(i)
		}
		return strings.Join(o, "\n") + "\n", strings.Join(n, "\n") + "\n"
	}
	grow := func(o string) string { return strings.Replace(o, "line 5\n", "line 5\nadded a\nadded b\n", 1) }
	o1, n1 := edit(9, 15, 40)
	o2, _ := edit()
	cases := map[string][2]string{
		"joined hunks":  {o1, n1},
		"growth":        {o2, strings.Replace(grow(o2), "line 40\n", "changed\n", 1)},
		"joined growth": {o2, strings.Replace(strings.Replace(grow(o2), "line 9\n", "x\n", 1), "line 50\n", "y\n", 1)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { checkHeaders(t, c[0], c[1]) })
	}
}

var testHeader = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)

func checkHeaders(t *testing.T, old, updated string) {
	t.Helper()
	d, err := unifiedDiff("f", []byte(old), []byte(updated))
	if err != nil {
		t.Fatal(err)
	}
	delta := 0
	hunks := 0
	for _, l := range strings.Split(d, "\n") {
		m := testHeader.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		hunks++
		oldStart, _ := strconv.Atoi(m[grpOldStart])
		newStart, _ := strconv.Atoi(m[grpNewStart])
		if newStart != oldStart+delta {
			t.Errorf("header %q: new start %d, want %d", l, newStart, oldStart+delta)
		}
		delta += countOf(m[grpNewCount]) - countOf(m[grpOldCount])
	}
	if hunks == 0 {
		t.Fatal("no hunk")
	}
}
