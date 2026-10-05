package build_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const testNamesSource = `/// A.
package a

/// Slot.
record Slot {
  /// Size.
  size: Int

  check fits: size < 10 else "big"
}

/// S.
let s: Slot = { size: 3 }

test "same" {
  expect s fails fits
}

test "same" {
  expect s fails missing
}
`

// EVALUATION.md §10.1, §10.3: `canon test` reports E5005 and E5004 as static errors.
func TestTestReportsNames(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(okProject), "p/a/a.canon": file(testNamesSource)}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Test(context.Background(), []string{"a"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{string(diag.E5005.Def().Code), string(diag.E5004.Def().Code)}
	var codes []string
	for _, f := range res.Static.List {
		codes = append(codes, string(f.Code))
	}
	if len(codes) != len(want) || codes[0] != want[0] || codes[1] != want[1] {
		t.Errorf("static findings %v, want %v", codes, want)
	}
}
