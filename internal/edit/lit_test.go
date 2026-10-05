package edit_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// typedV holds a field of each type API.md §8.2's table names.
const typedV = `/// V.
package v

/// A colour.
enum Color { red, green = "GREEN" }

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// Radius.
    r: Int
    /// Label.
    label: String = ""
  }
  /// A dot.
  dot
}

/// A part.
record Part {
  /// Its name.
  name: String
  /// A weight.
  w: Int = 0
}

/// A row.
record Row {
  /// Its code.
  code: Int
}

/// A tree.
record Node {
  /// Its children.
  kids: [Node] = []
}

/// A colour row.
record ByColor {
  /// Its colour.
  c: Color
}

/// A cell.
record Cell {
  /// On.
  on: Bool = false
  /// A count.
  n: Int8 = 0
  /// A ratio.
  f: Float = 0.5
  /// A small ratio.
  g: Float32 = 0.5
  /// A label.
  s: String = ""
  /// A delay.
  d: Duration = 0s
  /// A colour.
  c: Color = red
  /// Maybe a number.
  o: Int?
  /// A part.
  p: ref parts = a
  /// A row.
  row: ref rows = 7
  /// A shape.
  sh: Shape = dot
  /// Parts.
  ps: [Part] keyed by name = []
  /// Numbers.
  xs: [Int] = []
  /// Per colour.
  byC: {Color: Int} = {}
  /// Per name.
  byS: {String: Int} = {}
  /// Per number.
  byN: {Int: String} = {}
  /// Per part.
  byP: {ref parts: Int} = {}
  /// A colour or none.
  tag: Color | "none" = "none"
  /// A part.
  part: Part = { name: "x" }
  /// A colour row.
  bc: ref byColors = red
  /// A secret.
  secret: input String? from env "V_SECRET"
  /// A tree.
  node: Node = {}
}

/// Parts.
let parts: table Part = {
  a { name: "a" }
}

/// Rows.
let rows: [Row] keyed by code = [{ code: 7 }]

/// Colour rows.
let byColors: [ByColor] keyed by c = [{ c: red }]

/// A cell.
let cell: Cell = {}
`

var typedLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/v/v.canon":     file(typedV),
}

// fieldTypes are the static types of cell's fields, by field name.
func fieldTypes(t *testing.T) map[string]types.Type {
	t.Helper()
	p, err := build.Open(typedLaw, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	f := analyze(t, p, []string{"v"}, true)
	out := map[string]types.Type{}
	for _, name := range []string{"on", "n", "f", "g", "s", "d", "c", "o", "p", "row", "sh", "ps", "xs", "byC", "byS", "byN", "byP", "tag", "part", "bc", "node"} {
		typ, err := f.Type(resolve(t, f, "v:cell."+name))
		if err != nil {
			t.Fatal(err)
		}
		out[name] = typ
	}
	cell, err := f.Type(resolve(t, f, "v:cell"))
	if err != nil {
		t.Fatal(err)
	}
	out["cell"] = cell
	return out
}

// typeCase is a Lit typed as the field `field`; want is the typed value's text, "" for a
// ValueError.
type typeCase struct {
	field string
	lit   edit.Lit
	want  string
}

func checkTyped(t *testing.T, cases []typeCase) {
	t.Helper()
	ts := fieldTypes(t)
	for _, c := range cases {
		v, err := edit.Typer{Host: noHost{}}.Value(context.Background(), c.lit, ts[c.field])
		switch {
		case c.want == "" && !errors.Is(err, edit.ErrBadValue):
			t.Errorf("%s %#v: got %v, %v, want ErrBadValue", c.field, c.lit, v, err)
		case c.want != "" && err != nil:
			t.Errorf("%s %#v: %v", c.field, c.lit, err)
		case c.want != "" && v.CanonText() != c.want:
			t.Errorf("%s %#v = %s, want %s", c.field, c.lit, v.CanonText(), c.want)
		}
	}
}

// API.md V1: each constructor fits the types its row of §8.2 names, and nothing else.
func TestTypedConstructors(t *testing.T) {
	checkTyped(t, []typeCase{
		{"on", edit.Bool(true), "true"},
		{"on", edit.Int(1), ""},
		{"n", edit.Int(3), "3"},
		{"f", edit.Int(3), "3"}, // an Int is accepted as a Float (TYP-10)
		{"f", edit.Float(1.5), "1.5"},
		{"g", edit.Float(0.1), "0.1"},
		{"n", edit.Float(1.5), ""},
		{"s", edit.Str("hi"), "hi"},
		{"d", edit.Dur(1500 * time.Millisecond), "1s500ms"},
		{"d", edit.Dur(time.Microsecond), ""}, // not a whole number of milliseconds
		{"c", edit.Member("green"), "green"},
		{"c", edit.Member("GREEN"), ""}, // a Canon name, not a wire value
		{"tag", edit.Member("red"), "red"},
		{"tag", edit.Str("none"), "none"},
		{"p", edit.Key("a"), "a"},
		{"row", edit.IntKey(7), "7"},
		{"row", edit.Key("7"), ""},
		{"p", edit.IntKey(1), ""},
		{"o", edit.None{}, "none"},
		{"o", edit.Int(4), "4"}, // a T where a T? is expected (SPEC §5.6)
		{"n", edit.None{}, ""},
		{"sh", edit.Case{Name: "circle", Fields: edit.Obj{"r": edit.Int(2)}}, "circle{r: 2, label: }"},
		{"sh", edit.Case{Name: "dot"}, "dot"},
		{"sh", edit.Case{Name: "square"}, ""},
		{"xs", edit.List{edit.Int(1), edit.Int(2)}, "[1, 2]"},
		{"ps", edit.List{edit.Obj{"name": edit.Str("q")}}, `[Part{name: "q", w: }]`},
		{"part", edit.Obj{"name": edit.Str("q"), "w": edit.Int(2)}, `Part{name: "q", w: 2}`},
		{"byC", edit.Map{{Key: edit.Member("red"), Value: edit.Int(1)}}, "{red: 1}"},
		{"byS", edit.Map{{Key: edit.Key("k"), Value: edit.Int(1)}, {Key: edit.Str("j"), Value: edit.Int(2)}}, `{"k": 1, "j": 2}`},
		{"byN", edit.Map{{Key: edit.IntKey(3), Value: edit.Str("x")}}, `{3: "x"}`},
		{"byP", edit.Map{{Key: edit.Key("a"), Value: edit.Int(1)}}, "{a: 1}"},
		{"cell", edit.Obj{"xs": edit.List{edit.Str("x")}}, ""},
	})
}

// API.md V2: only the static type is checked; a refinement, a missing ref target or a key
// that is not in the collection wait for the re-check.
func TestTypedStaticTypeOnly(t *testing.T) {
	checkTyped(t, []typeCase{
		{"n", edit.Int(1000), "1000"}, // Int8's range is the re-check's
		{"p", edit.Key("nowhere"), "nowhere"},
		{"row", edit.IntKey(99), "99"},
	})
}

// API.md V3: an Obj leaves out fields to keep their defaults; a required field left out is
// the re-check's E3302, not a ValueError. An unknown field is a ValueError.
func TestTypedObjLeavesFieldsOut(t *testing.T) {
	checkTyped(t, []typeCase{
		{"part", edit.Obj{"w": edit.Int(2)}, "Part{name: , w: 2}"},
		{"part", edit.Obj{}, "Part{name: , w: }"},
		{"part", edit.Obj{"size": edit.Int(2)}, ""},
		{"sh", edit.Case{Name: "circle"}, "circle{r: , label: }"},
	})
}

// API.md V4: integer and ref key constructors never coerce.
func TestTypedNoCoercion(t *testing.T) {
	checkTyped(t, []typeCase{
		{"n", edit.Str("3"), ""},
		{"c", edit.Key("red"), ""},
		{"s", edit.Key("x"), ""},
		{"s", edit.Member("red"), ""},
		{"n", edit.IntKey(3), ""},
		{"p", edit.Str("a"), ""},
		{"byC", edit.Map{{Key: edit.Key("red"), Value: edit.Int(1)}}, ""},
	})
}

// noHost is a host that computes nothing: a default is a placeholder, which typing drops as an
// unwritten field, and every dereference fails.
type noHost struct{}

func (noHost) Default(_ context.Context, f *types.Field, _ wire.Instance, via *value.Prov) (value.Value, bool) {
	return &value.None{T: f.Type, P: via}, true
}

func (noHost) Deref(context.Context, *value.Ref) (*value.Record, bool) { return nil, false }

func (noHost) Bind(*value.Record, map[*types.Param]value.Value) {}

func (noHost) Cycle(context.Context, *value.Ref) {}

func (noHost) Reads(*types.Field, []*types.Field) []int { return nil }

func (noHost) Savepoint() func(undo bool) { return func(bool) {} }

// log-2026-09-29 M4 U4a: a Typer's host is required.
func TestTyperNeedsHost(t *testing.T) {
	ts := fieldTypes(t)
	if _, err := (edit.Typer{}).Value(context.Background(), edit.Int(1), ts["n"]); !errors.Is(err, edit.ErrNoHost) {
		t.Errorf("Value without a host: %v, want ErrNoHost", err)
	}
	if _, err := (edit.Typer{}).Key(context.Background(), edit.Int(1), ts["n"]); !errors.Is(err, edit.ErrNoHost) {
		t.Errorf("Key without a host: %v, want ErrNoHost", err)
	}
}

// API.md V2 (log-2026-09-29 M4 U4a): a Float32 overflow is the re-check's E3202, as an Int8's
// range is; NaN and the infinities are never values.
func TestTypedFloat32Overflow(t *testing.T) {
	ts := fieldTypes(t)
	for _, lit := range []edit.Lit{edit.Float(1e39), edit.Source("1e39")} {
		v, err := edit.Typer{Host: noHost{}}.Value(context.Background(), lit, ts["g"])
		if f, ok := v.(*value.Float); err != nil || !ok || f.V != 1e39 {
			t.Errorf("%#v = %v, %v, want 1e39 kept whole", lit, v, err)
		}
	}
	checkTyped(t, []typeCase{{"g", edit.Float(math.Inf(1)), ""}, {"f", edit.Float(math.NaN()), ""}})
}

// API.md V2 (log-2026-09-29 M4 U4a): keys are the re-check's, so a map given one key twice is
// typed as given.
func TestTypedDuplicateKeys(t *testing.T) {
	checkTyped(t, []typeCase{
		{"byC", edit.Map{{Key: edit.Member("red"), Value: edit.Int(1)}, {Key: edit.Member("red"), Value: edit.Int(2)}}, "{red: 1, red: 2}"},
		{"byC", edit.Source("{ red: 1, red: 2 }"), "{red: 1, red: 2}"},
		{"ps", edit.FromJSON(`[{"name": "a"}, {"name": "a"}]`), `[Part{name: "a", w: }, Part{name: "a", w: }]`},
	})
}

// API.md V1 (go.md §3): a Lit too deep, or holding itself, is a ValueError, never a stack overflow.
func TestTypedDepthIsCapped(t *testing.T) {
	ts := fieldTypes(t)
	deep := edit.Obj{}
	for range 600 {
		deep = edit.Obj{"kids": edit.List{deep}}
	}
	loop := edit.Obj{}
	loop["kids"] = edit.List{loop}
	for _, lit := range []edit.Lit{deep, loop, edit.Source(strings.Repeat("{ kids: [", 300) + strings.Repeat("] }", 300))} {
		_, err := edit.Typer{Host: noHost{}}.Value(context.Background(), lit, ts["node"])
		if !errors.Is(err, edit.ErrBadValue) || !strings.Contains(err.Error(), "nested too deep") {
			t.Errorf("%T Lit: %v, want the depth refusal", lit, err)
		}
	}
	shallow := edit.Obj{"kids": edit.List{edit.Obj{}}}
	checkTyped(t, []typeCase{{"node", shallow, "Node{kids: [Node{kids: }]}"}})
}

// API.md V1: an input field has no value to give (API.md §7.2 `input`), a detail of its own.
func TestTypedInputField(t *testing.T) {
	ts := fieldTypes(t)
	for _, lit := range []edit.Lit{edit.Obj{"secret": edit.Str("x")}, edit.Source(`{ secret: "x" }`)} {
		_, err := edit.Typer{Host: noHost{}}.Value(context.Background(), lit, ts["cell"])
		if !errors.Is(err, edit.ErrBadValue) || !strings.Contains(err.Error(), "an input field has no value to give: secret") {
			t.Errorf("%#v: %v, want the input refusal", lit, err)
		}
	}
}
