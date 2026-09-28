package views_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/views/typedef"
)

// pinned holds one field per type expression §12.3 writes that no other test pins.
const pinned = `package a

variant Shape {
  circle { r: Int }
  square { side: Int }
}

record Pt {
  x: Int
  y: Int
}

record Pinned {
  icon: asset("icons", ext: [png])
  positive: Int where it > 0
  byKey: [Pt] keyed by x
  rows: table Pt
  narrow: Float32
  short: Int16
  ratio: Float(0.0..1.0)
  round: Shape.circle
  rounds: [Shape.circle]

  fn twice(self) -> Int {
    return short * 2
  }
}

view Pinned {
  twice "Twice"
}

/// Pinned values.
let pins: stable table Pt = {}
`

// VIEWMODEL.md 12.3 type expressions (asset, predicate, keyedBy, table, int and float bits,
// maxExclusive, a case used as a type) and record methods named by a view.
func TestPinnedTypeExpressions(t *testing.T) {
	m := demo(t, pinned, "").model(t, demoPkg)
	fields := map[string]any{}
	for _, f := range canonical(t, m.Types["a.Pinned"].Fields).([]any) {
		fields[member(f, "name").(string)] = member(f, "type")
	}
	for _, c := range []struct{ field, want string }{
		{"icon", `{"kind":"asset","root":"icons","ext":["png"]}`},
		{"positive", `{"kind":"int","bits":64,"signed":true,"predicate":"it > 0"}`},
		{"byKey", `{"kind":"list","of":{"kind":"record","ref":"a.Pt"},"keyedBy":"x"}`},
		{"rows", `{"kind":"table","of":{"kind":"record","ref":"a.Pt"}}`},
		{"narrow", `{"kind":"float","bits":32}`},
		{"short", `{"kind":"int","bits":16,"signed":true,"min":-32768,"max":32767}`},
		{"ratio", `{"kind":"float","bits":64,"min":0,"max":1,"maxExclusive":true}`},
		{"round", `{"kind":"variant","ref":"a.Shape","case":"circle"}`},
		{"rounds", `{"kind":"list","of":{"kind":"variant","ref":"a.Shape","case":"circle"}}`},
	} {
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(fields[c.field], want) {
			t.Errorf("%s type:\n got %s\nwant %s", c.field, text(fields[c.field]), c.want)
		}
	}
	methods := canonical(t, m.Types["a.Pinned"].Methods)
	if want := decode(t, []byte(`[{"name":"twice","returns":{"kind":"int","bits":64,"signed":true}}]`)); !reflect.DeepEqual(methods, want) {
		t.Errorf("methods = %s, want %s", text(methods), text(want))
	}
}

// VIEWMODEL.md 12.3 `table`: a stable table's expression is marked stable.
func TestStableTableExpression(t *testing.T) {
	x := demo(t, pinned, "")
	s := typedef.New(context.Background(), typedef.Input{Program: x.a.Program()}, demoPkg)
	got := canonical(t, s.Expr(nil, x.letType(t, demoPkg, "pins")))
	if want := decode(t, []byte(`{"kind":"table","of":{"kind":"record","ref":"a.Pt"},"stable":true}`)); !reflect.DeepEqual(got, want) {
		t.Errorf("pins type = %s, want %s", text(got), text(want))
	}
}

// VIEWMODEL.md C14, C26 and 12.5 `case`: an asset is a file picker; a case used as a type is a
// variant control with its case fixed and no selector; a list of it is a table of that case.
func TestPinnedControls(t *testing.T) {
	x := demo(t, pinned, "")
	rec := x.named(t, demoPkg, "Pinned")
	r := x.resolver()
	for _, c := range []struct{ field, want string }{
		{"icon", `{"kind":"file","root":"icons","ext":["png"]}`},
		{"round", `{"kind":"variant","of":"a.Shape","case":"circle"}`},
		{"rounds", `{"kind":"table","of":"a.Shape","case":"circle"}`},
	} {
		got := canonical(t, r.Field(rec, field(t, rec, c.field)))
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s control:\n got %s\nwant %s", c.field, text(got), c.want)
		}
	}
}

// VIEWMODEL.md J12: a ref into a load.defines table names the element `$define`.
func TestDefineElement(t *testing.T) {
	m := examples(t).model(t, "resource.vocab")
	job := canonical(t, m.Types["resource.vocab.Item"].Fields[3])
	if got := member(job, "type", "of", "element"); got != "$define" {
		t.Errorf("Item.job element = %v, want $define", got)
	}
}
