package diag_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

type typeText string

func (t typeText) String() string { return string(t) }

type valueText string

func (v valueText) CanonText() string { return string(v) }

// ERRORS.md §2.2: a constructor takes the span, then typed arguments; E1703 takes a Message.
func TestConstructorsTakeTypedArguments(t *testing.T) {
	span := source.Span{File: 1, Start: 12, End: 20}
	built := []*diag.Builder{
		diag.E3002.At(span, typeText("Int"), typeText("String")),
		diag.E1703.AtType(span, "farm.title", diag.E3501.At(span, valueText(`"II_SYS_SYS_SCR_FARM3"`), "resource.vocab.items").Message()),
		diag.E1005.AtRoot(span, "resource"),
	}
	for i, b := range built {
		if b == nil {
			t.Errorf("constructor %d returned nil", i)
		}
	}
}
