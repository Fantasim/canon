package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// valueOf finds a value of the built packages.
func valueOf(pkgs []*ir.Package, pkg, name string) *ir.Value {
	for _, p := range pkgs {
		for _, v := range p.Values {
			if p.Name == pkg && v.Name == name {
				return v
			}
		}
	}
	return nil
}

// FINGERPRINT.md §7: stage E's Value.Schema of each vector's value, built from its checked source (types converted, `$` fns classified, `$fns` for the json emit's first value).
func TestSchemasOfTheVectors(t *testing.T) {
	w := newWorld(t)
	w.examples(t, "teamboard", "sovcommon", "pipeline", "resource", "balance", "game")
	for _, name := range []string{"testdata/schemas/flow.txtar", "testdata/schemas/fpdemo.txtar"} {
		cases, err := golden.Load(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range cases[0].Archive.Files {
			w.add(t, f.Name, f.Data)
		}
	}
	pkgs := w.build(t)
	vectors := []struct{ pkg, value, want string }{
		{"pipeline", "potions", "pipeline.Potion@f750790e"},
		{"teamboard", "statuses", "teamboard.Status@4aed3fde"},
		{"resource.events", "eventConfig", "resource.events.EventConfig@1ba50ac8"},
		{"resource.heistia", "heistia", "resource.heistia.HeistiaConfig@39fd8523"},
		{"balance.parity", "plan", "balance.parity.Plan@a2305fd5"},
		{"fpdemo", "skills", "fpdemo.Skill@ae120ca0"},
		{"teamboard", "deck", "teamboard.Deck@02af81fb"},
		{"teamboard", "assigneeMinRole", "sovcommon.roles.Role@dc935485"},
		{"flow", "statuses", "flow.Status@b330a789"},
		{"resource.adventurequest", "adventureQuests", "resource.adventurequest.AdventureQuestConfig@6dc7944d"},
	}
	for i, c := range vectors {
		v := valueOf(pkgs, c.pkg, c.value)
		switch {
		case v == nil:
			t.Errorf("vector %d: no value %s.%s", i+1, c.pkg, c.value)
		case v.Schema != c.want:
			text, err := ir.Fingerprint(&v.Type, nil)
			t.Errorf("vector %d: Schema %q, want %q\n%s%v", i+1, v.Schema, c.want, text, err)
		}
	}
}
