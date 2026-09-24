package gogen

import (
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// CODEGEN.md §3.5, decision 182: each kind of name-plan problem is its sentinel, and an unexported override names its item (the names themselves are ir's, tested there).
func TestNameError(t *testing.T) {
	cases := []struct {
		pr   ir.GoNameProblem
		want error
	}{
		{ir.GoNameProblem{Kind: ir.GoUnexported, Name: "heal", Origin: "p.R.heal"}, ErrName},
		{ir.GoNameProblem{Kind: ir.GoNotIdentifier, Name: "", Origin: "p.R._"}, ErrName},
		{ir.GoNameProblem{Kind: ir.GoCollision, Scope: "package", Name: "ToneA", First: "p.Tone.a", Origin: "p.Tone.A"}, ErrNameCollision},
	}
	for _, c := range cases {
		if err := nameError(c.pr); !errors.Is(err, c.want) {
			t.Errorf("%+v: got %v, want %v", c.pr, err, c.want)
		}
	}
	var d *DetailError
	if err := nameError(cases[0].pr); !errors.As(err, &d) || d.Subject != "p.R.heal" {
		t.Errorf("an unexported override must name p.R.heal: %v", err)
	}
}
