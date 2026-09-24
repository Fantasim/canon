package gogen

import (
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// go.md §3: every kind of name-plan problem is ErrMalformed, naming an unexported override's item.
func TestNameError(t *testing.T) {
	cases := []ir.GoNameProblem{
		{Kind: ir.GoUnexported, Name: "heal", Origin: "p.R.heal"},
		{Kind: ir.GoNotIdentifier, Name: "", Origin: "p.R._"},
		{Kind: ir.GoCollision, Scope: "package", Name: "ToneA", First: "p.Tone.a", Origin: "p.Tone.A"},
	}
	for _, pr := range cases {
		if err := nameError(pr); !errors.Is(err, ErrMalformed) {
			t.Errorf("%+v: got %v, want ErrMalformed", pr, err)
		}
	}
	var d *DetailError
	if err := nameError(cases[0]); !errors.As(err, &d) || d.Subject != "p.R.heal" {
		t.Errorf("an unexported override must name p.R.heal: %v", err)
	}
}
