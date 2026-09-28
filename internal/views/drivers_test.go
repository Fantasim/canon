package views_test

import (
	"reflect"
	"testing"
)

// routes reaches each `match` function through one route only: a function without a `match`,
// the same in a dependent map, and a parameterized record's argument (TYPES.md 11).
const routes = `package a

enum Goal { kill, collect, visit }

record Kind {
  goal: Goal
}

local let kinds: table Kind = {
  k1 { goal: kill }
  k2 { goal: visit }
}

type ViaWrap(k: Kind) = match k.goal {
  kill => String
  collect => String(1..)
  visit => String(..=3)
}

type Wrap(k: Kind) = ViaWrap(k) | "x"

type ViaKey(k: Kind) = match k.goal {
  kill => String
  collect => String(1..)
  visit => String(..=3)
}

type WrapKey(k: Kind) = ViaKey(k) | "x"

type ViaBind(k: Kind) = match k.goal {
  kill => String
  collect => Int
  visit => Never
}

type Target(g: Goal) = match g {
  kill => String
  collect => Int
  visit => Never
}

record Thing(k: Kind) {
  v: ViaBind(k)?
  t: Target(k.goal)?
}

record Holder {
  kind: ref kinds
  w: Wrap(kind)
  per: {k in kinds: WrapKey(k)}
  thing: Thing(kind)
  goals: {k in kinds: Target(k.goal)}
}
`

// VIEWMODEL.md J14, J11: a collection passed through a function without a `match` (expanded),
// through one in a dependent map, or through a parameterized record's argument gives the
// `match` function its drivers.
func TestDriversThroughRoutes(t *testing.T) {
	m := demo(t, routes, "").model(t, demoPkg)
	want := decode(t, []byte(`{"a:kinds":{"k1":"kill","k2":"visit"}}`))
	for _, fn := range []string{"a.ViaWrap", "a.ViaKey", "a.ViaBind"} {
		if got := canonical(t, m.Types[fn].Drivers); !reflect.DeepEqual(got, want) {
			t.Errorf("%s drivers = %s, want %s", fn, text(got), text(want))
		}
	}
}

// VIEWMODEL.md J13: a driver read down a parameter or a map key carries its path, as a field's.
func TestDriverPathOnParamAndKey(t *testing.T) {
	x := demo(t, routes, "")
	r := x.resolver()
	thing := x.named(t, demoPkg, "Thing")
	holder := x.named(t, demoPkg, "Holder")
	for _, c := range []struct {
		got  any
		want string
	}{
		{r.Field(thing, field(t, thing, "t")).On, `{"param":"k","path":["goal"]}`},
		{r.Field(holder, field(t, holder, "goals")).Value.On, `{"key":true,"path":["goal"]}`},
	} {
		if got := canonical(t, c.got); !reflect.DeepEqual(got, decode(t, []byte(c.want))) {
			t.Errorf("on = %s, want %s", text(got), c.want)
		}
	}
}

// VIEWMODEL.md D10, 12.5 `of`: a dependent map's cards pick from the key domain, and name no
// `of` when the value is neither a record, a variant nor a `match` function (a union here).
func TestDependentMapCardsOf(t *testing.T) {
	x := demo(t, routes, "")
	holder := x.named(t, demoPkg, "Holder")
	got := x.resolver().Field(holder, field(t, holder, "per"))
	if got.Of != "" || got.Source == nil || got.Source.Collection != "a:kinds" || !got.KeyFixed {
		t.Errorf("per control = %s, want cards over a:kinds without of", text(got))
	}
}
