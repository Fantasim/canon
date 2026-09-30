package load

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/testkit/winpaths"
)

// winVolume is how Windows names its volumes: filepath.VolumeName as the compiler reads it.
var winVolume = project.NewPaths(winpaths.Sep, winpaths.VolumeName).Volume

// WIRE.md §6.5, API.md §2.2: an absolute name's volume is set apart from its segments, so the walk over a link's real prefixes never cuts into it.
func TestSplitVolume(t *testing.T) {
	cases := []struct {
		abs  string
		vol  string
		segs []string
	}{
		{"/p/data", "", []string{"", "p", "data"}},
		{"//server/share/p/data", "//server/share", []string{"", "p", "data"}},
		{"C:/p/", "C:", []string{"", "p"}},
		{"//server/share/", "//server/share", []string{""}},
	}
	for _, c := range cases {
		vol, segs := splitVolume(c.abs, winVolume)
		if vol != c.vol || !slices.Equal(segs, c.segs) {
			t.Errorf("splitVolume(%q) = %q, %q; want %q, %q", c.abs, vol, segs, c.vol, c.segs)
		}
	}
}

// WIRE.md §6.5: a bare volume is its own real path; "C:" alone is a drive's working directory, so it is never resolved.
func TestRealPrefixOfBareVolume(t *testing.T) {
	var l Loader
	if got := l.realPrefix("C:", []string{""}); got != "C:" {
		t.Errorf("realPrefix of a bare volume = %q", got)
	}
	if got := l.realPrefix("//server/share", []string{""}); got != "//server/share" {
		t.Errorf("realPrefix of a bare share = %q", got)
	}
}
