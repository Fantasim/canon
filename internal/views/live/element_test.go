package live_test

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/live"
)

// elementLaw holds each collection ElementAt names: a table, a plain list of copies of its
// entries, a keyed list and a map.
const elementLaw = `package a

record Pt {
  name: String
}

let pts: table Pt = {
  p1 { name: "A" }
  p2 { name: "B" }
}

let copies: [Pt] = [pts.p2, pts.p1]

let keyed: [Pt] keyed by name = [{ name: "k" }]

let byName: {String: Int} = { "x y": 1 }
`

// forced analyzes elementLaw and returns the value of a name, and the analysis.
func forced(t *testing.T) (func(string) value.Value, *build.Analysis) {
	t.Helper()
	fsys := mapFS{
		"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon":     &fstest.MapFile{Data: []byte(elementLaw)},
	}
	p, err := build.Open(fsys, lawDir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil || a.Result().Summary.Errors != 0 {
		t.Fatalf("Analyze: %v %+v", err, a.Result().List)
	}
	force := func(name string) value.Value {
		v, ok := a.Force(eval.Root{Pkg: "a", Name: name})
		if !ok {
			t.Fatalf("%s not computed", name)
		}
		return v
	}
	return force, a
}

// API.md P8, API.md V7, VIEWMODEL.md S9: an element is named by its key only
// in a table, a keyed list or a map; in a plain list, a copy of a table entry included, by `#<n>`
// from 1, with its `index` magic name; past the end, nothing.
func TestElementAt(t *testing.T) {
	force, _ := forced(t)
	cases := []struct {
		value string
		names []string
		index bool
	}{
		{"pts", []string{"p1", "p2"}, false},
		{"copies", []string{"#1", "#2"}, true},
		{"keyed", []string{"k"}, true},
		{"byName", []string{"x y"}, false},
	}
	for _, c := range cases {
		coll := force(c.value)
		var names []string
		for i := 0; ; i++ {
			e, ok := live.ElementAt(coll, i)
			if !ok {
				break
			}
			names = append(names, e.Name)
			if (e.Magic.Index != nil) != c.index {
				t.Errorf("%s[%d]: index magic %v", c.value, i, e.Magic.Index)
			}
		}
		if !slices.Equal(names, c.names) {
			t.Errorf("%s: names %q, want %q", c.value, names, c.names)
		}
	}
	if _, ok := live.ElementAt(force("pts"), -1); ok {
		t.Error("a negative position names an element")
	}
}

// API.md P8, API.md V6: the headings of a plain list of table entries are keyed `[n]`.
func TestCopiesHeadings(t *testing.T) {
	force, a := forced(t)
	res, err := live.Evaluate(context.Background(), live.Input{Program: a.Program(), Eval: standIn{info: a.Program().Info}}, live.Target{Value: force("copies"), Name: "copies"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Headings["[0]"].Title.Value != "#1" || res.Headings["[1]"].Title.Value != "#2" || len(res.Headings) != 2 {
		t.Errorf("headings %+v", res.Headings)
	}
}
