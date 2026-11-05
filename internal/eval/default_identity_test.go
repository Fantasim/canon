package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// defaultFieldLaw holds keyed lists no ref targets in records a field default builds: holder.p,
// an applied record, and h.q, a plain one, read through a let path; g.q is written out.
const defaultFieldLaw = `-- a/a.canon --
/// A.
package a

/// S.
record S {
  /// Name.
  name: String
}

/// Ev.
record Ev {
  /// Max.
  max: Int = 3
}

/// Applied.
record P(e: Ev) {
  /// Plain keyed list.
  pl: [S] keyed by name = []
}

/// Plain.
record Pl {
  /// Plain keyed list.
  pl: [S] keyed by name = []
}

/// Holder.
record Holder {
  /// Ev.
  ev: Ev = {}
  /// P, by default.
  p: P(ev) = { pl: [{ name: "x" }] }
  /// Pl, by default.
  q: Pl = { pl: [{ name: "x" }] }
}

/// Defaults only.
let h: Holder = {}

/// Written out.
let g: Holder = { q: { pl: [{ name: "y" }] } }

/// An entry of h.q.pl as a ref.
let e: ref h.q.pl = h.q.pl.x

/// The entry and the ref are one.
let same: Bool = h.q.pl.x == e

/// An entry of g.q.pl as a ref.
let f: ref g.q.pl = g.q.pl.y
`

// EVALUATION.md §4.2, TYPES.md §6.3, §10.2, DECISIONS 315: a default's copy is its record's instance.
func TestDefaultFieldInstance(t *testing.T) {
	b := runBuild(t, fromArchive(t, txtar.Parse([]byte(defaultFieldLaw))), eval.Options{})
	for _, f := range b.bags["a"].Findings() {
		if f.Severity == diag.Error {
			t.Errorf("TYPES.md §10.2: a: %s %s", f.Code, f.Message)
		}
	}
	h := rootRecord(t, b, "h")
	for _, field := range []string{"p", "q"} {
		rec, _ := fieldValue(h, field).(*value.Record)
		if rec == nil {
			t.Fatalf("h.%s: no record", field)
		}
		el := entryOf(rec, "pl", value.Key{S: "x"})
		if el == nil || el.Ident == nil || el.Ident.Owner == nil {
			t.Fatalf("TYPES.md §6.3: h.%s.pl.x: no element owned by a record", field)
		}
		if !sameInstance(b.ev, el.Ident.Owner, rec) {
			t.Errorf("EVALUATION.md §4.2: h.%s.pl.x's owner %p is not h.%s %p's instance", field, el.Ident.Owner, field, rec)
		}
	}
	checkOwners(t, b, "defaultFieldLaw")
	for _, c := range []struct{ name, want string }{{"e", "x"}, {"f", "y"}} {
		if r, ok := b.values[eval.Root{Pkg: "a", Name: c.name}].(*value.Ref); !ok || r.Key.S != c.want {
			t.Errorf("TYPES.md §10.2: a.%s = %v, want a ref keyed %s", c.name, b.values[eval.Root{Pkg: "a", Name: c.name}], c.want)
		}
	}
	if same, ok := b.values[eval.Root{Pkg: "a", Name: "same"}].(*value.Bool); !ok || !same.V {
		t.Errorf("TYPES.md §7.5: a.same = %v, want true", b.values[eval.Root{Pkg: "a", Name: "same"}])
	}
}

// letPathLaw is defaultFieldLaw with one, a Pl written out, two its spread, and three a spread
// that writes its own pl.
const letPathLaw = defaultFieldLaw + `
/// One.
let one: Pl = { pl: [{ name: "x" }] }

/// Its spread.
let two: Pl = { ...one }

/// A spread with its own entries.
let three: Pl = { ...one, pl: [{ name: "x" }] }

/// A ref of two's pl.
let rb: ref two.pl = one.pl.x
`

// TYPES.md §10.3, DECISIONS 315, 317, 410: an entry converts to, and equals, a let path's ref when its field holds it.
func TestLetPathEntries(t *testing.T) {
	for _, c := range []struct {
		name, let, want string
		e3503           bool
	}{
		{"default", "let r: ref h.q.pl = h.q.pl.x", "", false},
		{"written out", "let r: ref g.q.pl = g.q.pl.y", "", false},
		{"another instance", "let r: ref h.q.pl = g.q.pl.y", "", true},
		{"spread of the source", "let r: ref two.pl = one.pl.x", "", false},
		{"source of the spread", "let r: ref one.pl = two.pl.x", "", false},
		{"spread writing its own entries", "let r: ref three.pl = one.pl.x", "", true},
		{"source of those entries", "let r: ref one.pl = three.pl.x", "", true},
		{"equal, held", "let r: Bool = one.pl.x == rb", "true", false},
		{"equal, not held", "let r: Bool = three.pl.x == rb", "false", false},
		{"equal, not held, ref first", "let r: Bool = rb == three.pl.x", "false", false},
		{"in", "let r: Bool = three.pl.x in [rb]", "false", true},
		{"contains", "let r: Bool = [rb].contains(three.pl.x)", "false", true},
		{"indexOf", "let r: Int? = [rb].indexOf(three.pl.x)", "none", true},
		{"indexOf, held", "let r: Int? = [rb].indexOf(two.pl.x)", "0", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			law := letPathLaw + "\n/// R.\n" + c.let + "\n"
			b := runBuild(t, fromArchive(t, txtar.Parse([]byte(law))), eval.Options{})
			got := false
			for _, f := range b.bags["a"].Findings() {
				switch {
				case f.Code == diag.E3503.Def().Code:
					got = true
				case f.Severity == diag.Error:
					t.Errorf("a: %s %s", f.Code, f.Message)
				}
			}
			if got != c.e3503 {
				t.Errorf("TYPES.md §10.3: %s: %s %v, want %v", c.let, diag.E3503.Def().Code, got, c.e3503)
			}
			if v := b.values[eval.Root{Pkg: "a", Name: "r"}]; c.want != "" && (v == nil || v.CanonText() != c.want) {
				t.Errorf("DECISIONS 315: %s = %v, want %s", c.let, v, c.want)
			}
		})
	}
}
