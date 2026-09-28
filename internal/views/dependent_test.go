package views_test

import (
	"reflect"
	"testing"
)

// dependents holds dependent fields driven by an enum, a Bool and a ref, a union over a type
// function without a `match`, and a dependent map (TYPES.md 11).
const dependents = `package a

enum Goal { kill, collect, visit }

type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
  visit => Never
}

type Name(g: Goal) = match g {
  kill => String
  collect => String(1..)
  visit => String(..=3)
}

type Pick(g: Goal) = Name(g) | "any"

type Reward(item: Bool) = match item {
  false => Int(1..)
  true => String(1..)
}

record Kind {
  goal: Goal
}

local let kinds: table Kind = {
  k1 { goal: kill }
  k2 { goal: visit }
  retired k3 { goal: collect }
}

type ByKind(k: Kind) = match k.goal {
  kill => String
  collect => Int
  visit => Never
}

record Objective {
  goal: Goal
  target: Target(goal)?
  pick: Pick(goal)
  item: Bool
  reward: Reward(item)
  kind: ref kinds
  by: ByKind(kind)?
  per: {k in kinds: ByKind(k)}
}
`

// VIEWMODEL.md C16, D6, D8, C15, J11, D10: one control per branch in branch order and the
// driver of the `match`; a function without one expanded, literals pinned; a dependent map is
// cards whose keys are fixed.
func TestDependentControls(t *testing.T) {
	x := demo(t, dependents, "")
	rec := x.named(t, demoPkg, "Objective")
	r := x.resolver()
	for _, c := range []struct{ field, want, rule string }{
		{"target", `{"kind":"dependent","fn":"a.Target","on":{"field":"goal"},"optional":{"unset":"clear"},
			"branches":[{"kind":"input","minLen":1},{"kind":"number","min":1},{"kind":"never"}]}`, "C16 D6 D8"},
		{"pick", `{"kind":"dependent","fn":"a.Name","on":{"field":"goal"},"pinned":["any"],
			"branches":[{"kind":"input"},{"kind":"input","minLen":1},{"kind":"input","maxLen":3}]}`, "J11 C15"},
		{"reward", `{"kind":"dependent","fn":"a.Reward","on":{"field":"item"},
			"branches":[{"kind":"number","min":1},{"kind":"input","minLen":1}]}`, "C16 Bool"},
		{"by", `{"kind":"dependent","fn":"a.ByKind","on":{"field":"kind"},"optional":{"unset":"clear"},
			"branches":[{"kind":"input"},{"kind":"number"},{"kind":"never"}]}`, "C16 ref driver"},
		{"per", `{"kind":"cards","of":"a.ByKind","keyFixed":true,"source":{"collection":"a:kinds"},
			"keyControl":{"kind":"select","source":{"collection":"a:kinds"}},
			"value":{"kind":"dependent","fn":"a.ByKind","on":{"key":true},
				"branches":[{"kind":"input"},{"kind":"number"},{"kind":"never"}]}}`, "D10 J13 key"},
	} {
		got := canonical(t, r.Field(rec, field(t, rec, c.field)))
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s %s:\n got %s\nwant %s", c.rule, c.field, text(got), c.want)
		}
	}
}

// VIEWMODEL.md J11, J14 (typeFunction): branches in `match` order and, for each collection whose
// entries the program passes, each entry's discriminant, retired ones included; a function
// without a `match` is no definition, expanded where applied.
func TestTypeFunctionDefinitions(t *testing.T) {
	m := demo(t, dependents, "").model(t, demoPkg)
	for _, c := range []struct{ name, want string }{
		{"a.Target", `{"kind":"typeFunction","name":"Target","params":[{"name":"g","type":{"kind":"enum","ref":"a.Goal"}}],
			"select":"","branches":[{"match":["kill"],"type":{"kind":"string","minLen":1}},
			{"match":["collect"],"type":{"kind":"int","bits":64,"signed":true,"min":1}},{"match":["visit"],"type":{"kind":"never"}}],
			"drivers":{}}`},
		{"a.Reward", `{"kind":"typeFunction","name":"Reward","params":[{"name":"item","type":{"kind":"bool"}}],
			"select":"","branches":[{"match":["false"],"type":{"kind":"int","bits":64,"signed":true,"min":1}},
			{"match":["true"],"type":{"kind":"string","minLen":1}}],"drivers":{}}`},
		{"a.ByKind", `{"kind":"typeFunction","name":"ByKind","params":[{"name":"k","type":{"kind":"record","ref":"a.Kind"}}],
			"select":"goal","branches":[{"match":["kill"],"type":{"kind":"string"}},
			{"match":["collect"],"type":{"kind":"int","bits":64,"signed":true}},{"match":["visit"],"type":{"kind":"never"}}],
			"drivers":{"a:kinds":{"k1":"kill","k2":"visit","k3":"collect"}}}`},
	} {
		got := canonical(t, m.Types[c.name])
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, text(got), c.want)
		}
	}
	if _, ok := m.Types["a.Pick"]; ok {
		t.Error("a.Pick, a function without a match, is a definition; J11 expands it")
	}
	fields := canonical(t, m.Types["a.Objective"].Fields)
	want := decode(t, []byte(`{"kind":"union","of":{"kind":"dependent","fn":"a.Name","on":{"field":"goal"}},"literals":["any"]}`))
	if got := member(fields.([]any)[2], "type"); !reflect.DeepEqual(got, want) {
		t.Errorf("pick's type = %s, want %s", text(got), text(want))
	}
}

// VIEWMODEL.md J13: an argument read down an earlier field's fields is that field and its path:
// features/dependent's `bonus: Reward(place.items)`.
func TestDriverPath(t *testing.T) {
	x := examples(t)
	obj := x.named(t, "features.dependent", "Objective")
	want := decode(t, []byte(`{"field":"place","path":["items"]}`))
	if got := canonical(t, x.resolver().Field(obj, field(t, obj, "bonus")).On); !reflect.DeepEqual(got, want) {
		t.Errorf("bonus control on = %s, want %s", text(got), text(want))
	}
	typ := member(canonical(t, x.model(t, "features.dependent").Types["features.dependent.Objective"].Fields[5]), "type", "on")
	if !reflect.DeepEqual(typ, want) {
		t.Errorf("bonus type on = %s, want %s", text(typ), text(want))
	}
}
