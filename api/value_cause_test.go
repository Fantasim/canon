package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// API.md R6 (DECISIONS 411): a constant read by a function alone, whose fold divides by zero, is
// ErrNoValue wrapping that finding at its declaration.
func TestValueCauseFoldedConst(t *testing.T) {
	p, _ := openLaw(t, map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon": "/// A.\npackage a\n\nlocal const ZERO = 0\n\nlocal const C = 1 / ZERO\n\n" +
			"/// Reads C.\nexport fn f() -> Int {\n  return C\n}\n",
	})
	_, err := p.Value(context.Background(), "a:C")
	var perr *canon.PathError
	if !errors.As(err, &perr) || !errors.Is(err, canon.ErrNoValue) {
		t.Fatalf("Value(a:C): %v, want ErrNoValue", err)
	}
	if len(perr.Findings) != 1 || !isCode(diag.E4102.Def().Code)(perr.Findings[0]) || perr.Findings[0].File != "a/a.canon" || perr.Findings[0].Line != 6 {
		t.Errorf("Value(a:C) findings %+v, want the fold's division by zero at a/a.canon:6", perr.Findings)
	}
}
