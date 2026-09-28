package views_test

import (
	"reflect"
	"testing"
)

// outerKeys passes a map key into a parameterized record that holds maps of its own: one
// driving by the outer key, one whose key reuses the outer key's name.
var outerKeys = map[string]string{
	"outer": `package a
enum Goal { a, b }
enum Color { red, blue }
record Kind {
  id: String
  goal: Goal
}
record Paint {
  id: String
  color: Color
}
let kinds: [Kind] keyed by id = [{ id: "k1", goal: a }, { id: "k2", goal: b }]
let paints: [Paint] keyed by id = [{ id: "p1", color: red }, { id: "p2", color: blue }]
type F(k: Kind) = match k.goal {
  a => String
  b => Int
}
record Inner(k: Kind) {
  per: {j in paints: F(k)} = {}
}
record Top {
  m: {k in kinds: Inner(k)}
}
`,
	"sameName": `package a
enum Goal { a, b }
enum Color { red, blue }
record Kind {
  id: String
  goal: Goal
}
record Paint {
  id: String
  color: Color
}
let kinds: [Kind] keyed by id = [{ id: "k1", goal: a }, { id: "k2", goal: b }]
let paints: [Paint] keyed by id = [{ id: "p1", color: red }, { id: "p2", color: blue }]
type F(k: Kind) = match k.goal {
  a => String
  b => Int
}
type G(p: Paint) = match p.color {
  red => String
  blue => Int
}
record Inner(q: Kind) {
  per: {k in paints: G(k)} = {}
  own: F(q)?
}
record Top {
  m: {k in kinds: Inner(k)}
}
`,
}

// VIEWMODEL.md J13, J14: a map key passed into a record is read against the map that binds it,
// never against a map inside the record, whatever the inner key's name.
func TestOuterMapKeyDrivers(t *testing.T) {
	kinds := `{"a:kinds":{"k1":"a","k2":"b"}}`
	paints := `{"a:paints":{"p1":"red","p2":"blue"}}`
	for _, c := range []struct{ src, fn, want string }{
		{"outer", "a.F", kinds},
		{"sameName", "a.F", kinds},
		{"sameName", "a.G", paints},
	} {
		m := demo(t, outerKeys[c.src], "").model(t, demoPkg)
		if got := canonical(t, m.Types[c.fn].Drivers); !reflect.DeepEqual(got, decode(t, []byte(c.want))) {
			t.Errorf("%s %s drivers = %s, want %s", c.src, c.fn, text(got), c.want)
		}
	}
}
