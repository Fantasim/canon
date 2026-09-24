package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// litUnionSource has a string-literal union over a ref whose literal is no key of the table.
const litUnionSource = `package a

/// An item.
record Item {
  /// Its power.
  power: Int = 1
}

/// The items.
let items: table Item = { sword {} }

/// A weight target.
type Target = ref Item | "all"

/// The weights.
let weights: {Target: Int} = { sword: 1, "all": 2 }

/// One target.
let one: Target = "all"

/// A key.
let two: Target = "sword"
`

// TYPES.md §13.2 (TYP-09): "all" is the union's literal, never a key of items; "sword" is a key.
func TestLitUnionLiteralWins(t *testing.T) {
	b := checkBuilt(t, litUnionSource)
	if len(b.codes) != 0 {
		t.Fatalf("findings:\n%s", b.out)
	}
	for _, s := range nodesOf[*syntax.StringLit](b.file) {
		text := s.Parts[0].Text
		_, isKey := b.prog.Info.Keys[s]
		if want := text != "all"; isKey != want {
			t.Errorf("%q recorded as a key: %v, want %v", text, isKey, want)
		}
	}
	res := buildOne(t, litUnionSource)
	if errs := errorCodes(res.List); len(errs) != 0 {
		t.Errorf("a build reports %v, want no error", errs)
	}
	for _, fd := range res.List {
		if fd.Code == diag.E3501.Def().Code {
			t.Errorf("a key the literal wins: %s", fd.Message)
		}
	}
}
