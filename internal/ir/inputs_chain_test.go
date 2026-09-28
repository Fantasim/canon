package ir_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// chainSource declares inputs through alias chains: p through two levels, each with its own
// pattern, plus a length; q through a chain that repeats its inner pattern.
const chainSource = `package a

/// A lower-case code.
type Code = String(/^[a-z]+$/)

/// A short code.
type Short = Code(/^.{1,4}$/)

/// The same pattern again.
type Again = Code(/^[a-z]+$/)

/// Settings.
record S {
  /// A short code.
  p: input Short(..=3)? from env "P"
  /// A code, its pattern written twice.
  q: input Again? from env "Q"
}

/// The settings.
let v: S = {}

emit go { out: "@features/a", package: "a" }
`

// TestInputPatternsThroughAliasChain is TYPES.md §7.4 ("both are checked"), EVALUATION.md §11.3 step 3 and DECISIONS 225: an input's IR keeps every pattern of its alias chain in declaration order, innermost first, a repeated one once, and the intersection of its ranges.
func TestInputPatternsThroughAliasChain(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(chainSource))
	w.add(t, "a.v.json", []byte("{}"))
	var fields []*ir.Field
	for _, pkg := range w.build(t) {
		for _, typ := range pkg.Types {
			if r, ok := typ.(*ir.Record); ok && r.Name == "S" {
				fields = r.Fields
			}
		}
	}
	if got := w.findings(t); !strings.HasPrefix(got, noFindings) {
		t.Fatalf("findings:\n%s", got)
	}
	if len(fields) != len(chainWant) {
		t.Fatalf("record S has %d fields, want %d", len(fields), len(chainWant))
	}
	for i, want := range chainWant {
		var got []string
		for _, re := range fields[i].Patterns {
			got = append(got, re.String())
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: patterns %q, want %q", fields[i].Name, got, want)
		}
	}
	if r := fields[0].Range; r == nil || !r.HasHi || r.Hi.I != 3 || !r.HiIncluded {
		t.Errorf("range %+v, want ..=3", r)
	}
}

// chainWant are record S's fields' patterns, in field order.
var chainWant = [][]string{{`^[a-z]+$`, `^.{1,4}$`}, {`^[a-z]+$`}}
