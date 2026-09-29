package i18n_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/i18n"
)

const (
	badcachePkg    = "j4e"
	badcacheKey    = "Nope.n"
	badcacheHead   = "package j4e\n\n/// Fine.\nrecord Good {\n  /// Name.\n  name: String\n}\n"
	badcacheBroken = badcacheHead + "\nrecord Item {\n  n: Int\n"
	badcacheFixed  = badcacheHead + "\nrecord Item {\n  n: Int\n}\n"
)

// I18N.md F4, IMPLEMENTATION-PLAN §7.6: one BadCache over a broken program, then over the same file fixed, answers as no cache does.
func TestBadCacheFollowsTheFileFixed(t *testing.T) {
	cache := &i18n.BadCache{}
	for _, st := range []struct {
		name, src string
		silent    bool
	}{{"broken", badcacheBroken, true}, {"fixed", badcacheFixed, false}, {"broken again", badcacheBroken, true}} {
		c := checkFiles(t, map[string]string{
			"j4e/j4e.canon":    st.src,
			"j4e/j4e.fr.canon": "package j4e\ntranslation fr\n\nNope.n \"autre chose\"\n",
		})
		cold := i18n.Check(c.prog, fixtureProject(), c.bags, emitsView(c.prog))[badcachePkg].Catalogue.Resolve(badcacheKey)
		warm := i18n.CheckWith(c.prog, fixtureProject(), c.bags, emitsView(c.prog), cache)[badcachePkg].Catalogue.Resolve(badcacheKey)
		if warm != cold || warm.Silent != st.silent {
			t.Errorf("%s: cached resolution %+v, cold %+v, want silent %t", st.name, warm, cold, st.silent)
		}
	}
}
