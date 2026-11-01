package project_test

import (
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

// WIRE.md §6.5, DECISIONS 419: ResolvedAs gives child's recorded answer again and resolves any other name afresh.
func TestResolvedAs(t *testing.T) {
	errRecorded := errors.New("recorded")
	fresh := func(name string) (string, error) { return "fresh:" + name, nil }
	resolve := project.ResolvedAs(fresh, "d/c", "recorded-target", errRecorded)
	if got, err := resolve("d/c"); got != "recorded-target" || !errors.Is(err, errRecorded) {
		t.Errorf("child: got %q, %v; want the recorded answer", got, err)
	}
	if got, err := resolve("d"); got != "fresh:d" || err != nil {
		t.Errorf("other name: got %q, %v; want a fresh resolution", got, err)
	}
}

// WIRE.md §2.1: CutRoot splits "@name/rest"; an unrooted path is whole and a bare "@name" is the root itself.
func TestCutRoot(t *testing.T) {
	cases := []struct {
		in, name, rest string
		rooted         bool
	}{
		{"@art/a/b.png", "art", "a/b.png", true},
		{"@art", "art", "", true},
		{"@", "", "", true},
		{"a/b", "", "a/b", false},
	}
	for _, c := range cases {
		name, rest, rooted := project.CutRoot(c.in)
		if name != c.name || rest != c.rest || rooted != c.rooted {
			t.Errorf("CutRoot(%q) = %q, %q, %v; want %q, %q, %v", c.in, name, rest, rooted, c.name, c.rest, c.rooted)
		}
	}
}
