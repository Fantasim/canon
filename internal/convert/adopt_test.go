package convert_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/convert"
	"github.com/fantasim/canonlang/internal/project"
)

// CLI.md §3.10 step 4, DECISIONS 269.
func TestAdoptOut(t *testing.T) {
	p := project.New("acme", project.Version{Major: 0, Minor: 1})
	p.Roots = []project.Root{{Name: "game", Path: "../game"}, {Name: "gen", Path: "gen"}, {Name: "web", Path: "../web"}}
	for _, c := range []struct {
		name   string
		out    []string
		list   bool
		legacy string
		want   []string
		err    error
	}{
		{"single out", []string{"@web/items.json"}, false, "@game/items.json", []string{"@game/items.json"}, nil},
		{"added", []string{"@web/items.json"}, true, "@game/items.json", []string{"@web/items.json", "@game/items.json"}, nil},
		{"added under the project", []string{"@web/items.json", "@gen/items.json"}, true, "data/items.json",
			[]string{"@web/items.json", "@gen/items.json", "data/items.json"}, nil},
		{"already an entry", []string{"@web/items.json", "@game/items.json"}, true, "@game/items.json",
			[]string{"@web/items.json", "@game/items.json"}, nil},
		{"already an entry, written otherwise", []string{"@web/items.json", "../gen/x/../items.json"}, true, "@gen/items.json",
			[]string{"@web/items.json", "../gen/x/../items.json"}, nil},
		{"same root", []string{"@game/old/items.json"}, true, "@game/items.json", nil, convert.ErrAdoptRoot},
		{"same root, the project", []string{"out/items.json"}, true, "data/items.json", nil, convert.ErrAdoptRoot},
		{"directories", []string{"@web/data/"}, true, "@game/items.json", nil, convert.ErrAdoptForm},
		{"unresolved", []string{"@web/items.json"}, true, "@nowhere/items.json", nil, convert.ErrAdoptPath},
	} {
		got, err := convert.AdoptOut(p, "items", c.out, c.list, c.legacy)
		if !errors.Is(err, c.err) || !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, %v; want %v, %v", c.name, got, err, c.want, c.err)
		}
	}
}
