package progen_test

import (
	"bytes"
	"slices"
	"testing"
)

// TestRetiredMemberSiteFollowsItsProject: a blank line before the use keeps the site (TYPES.md §8.1).
func TestRetiredMemberSiteFollowsItsProject(t *testing.T) {
	c := examples(t)
	i := slices.IndexFunc(c.targets, func(tg target) bool { return len(retiredMemberUse(tg)) > 0 })
	if i < 0 {
		t.Fatalf("no retired-member site in the examples%s", refsTrouble())
	}
	tg := c.targets[i]
	sites := retiredMemberUse(tg)
	use := sites[0].Edits[sites[0].Focus].Start
	line := bytes.LastIndexByte(tg.src[:use], '\n') + 1
	q := c.project.Clone()
	q.Set(tg.path, slices.Concat(tg.src[:line], []byte("\n"), tg.src[line:]))
	moved := targetsOf(q, []string{tg.pkg})
	j := slices.IndexFunc(moved, func(m target) bool { return m.path == tg.path })
	if got := len(retiredMemberUse(moved[j])); got != len(sites) {
		t.Errorf("%s with a blank line before the use: %d sites, want %d%s", tg.path, got, len(sites), refsTrouble())
	}
}
