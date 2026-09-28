package views_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/views"
)

// scanDeadline bounds a build that would otherwise never finish: the test fails, it never hangs.
const scanDeadline = 20 * time.Second

// selfApplied has a record applying itself with an argument read down a ref, directly and
// through a second record.
var selfApplied = map[string]string{
	"self": `package a
enum Goal { a, b }
record Kind {
  id: String
  goal: Goal
  parent: ref kinds
}
let kinds: [Kind] keyed by id = [{ id: "x", goal: a, parent: "x" }]
type F(k: Kind) = match k.goal {
  a => String
  b => Int
}
record Node(k: Kind) {
  v: F(k)?
  next: [Node(k.parent)] = []
}
record Top {
  kind: ref kinds
  n: Node(kind)
}
`,
	"mutual": `package a
enum Goal { a, b }
record Kind {
  id: String
  goal: Goal
  parent: ref kinds
}
let kinds: [Kind] keyed by id = [{ id: "x", goal: a, parent: "x" }]
type F(k: Kind) = match k.goal {
  a => String
  b => Int
}
record A(k: Kind) {
  v: F(k)?
  b: [B(k.parent)] = []
}
record B(k: Kind) {
  a: [A(k.parent)] = []
}
record Top {
  kind: ref kinds
  n: A(kind)
}
`,
}

// SPEC.md 1 (always finishes), VIEWMODEL.md J14: a record applying itself, or two records
// applying each other, down a ref finishes, and F's drivers still read kinds.
func TestSelfAppliedRecordsFinish(t *testing.T) {
	want := decode(t, []byte(`{"a:kinds":{"x":"a"}}`))
	for _, name := range []string{"self", "mutual"} {
		x := demo(t, selfApplied[name], "")
		ctx, cancel := context.WithTimeout(context.Background(), scanDeadline)
		m, err := views.Build(ctx, views.Input{
			Program: x.a.Program(), Package: demoPkg, Force: x.a.Force, Fold: eval.NewFolder(x.bags, eval.Options{}),
		})
		cancel()
		if err != nil {
			t.Fatalf("%s: Build = %v, want it to finish", name, err)
		}
		if got := canonical(t, m.Types["a.F"].Drivers); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: F drivers = %s, want %s", name, text(got), text(want))
		}
	}
}
