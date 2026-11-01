package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// CODEGEN.md §5.15 T19: the side of a bound in words, lower case and for a sentence, and the subject: the field's Canon name, each step of the path before it (the deepest first), a length's prefix before all; the three targets write these bytes.
func TestBoundTextWords(t *testing.T) {
	for _, c := range []struct {
		side           ir.BoundSide
		text, sentence string
		step           []ir.BoundStep
		length         bool
		level          types.Kind
		subject        string
	}{
		{side: ir.SideMin, text: "lower bound", sentence: "Lower bound", level: types.Int, subject: "age"},
		{side: ir.SideMax, text: "upper bound", sentence: "Upper bound", level: types.Int, subject: "age"},
		{side: ir.SideEnd, text: "exclusive upper bound", sentence: "Exclusive upper bound", step: []ir.BoundStep{ir.StepKey, ir.StepElem}, level: types.Int, subject: "each element of each key of age"},
		{side: ir.SideMin, text: "lower bound", sentence: "Lower bound", length: true, level: types.String, subject: "the byte length of age"},
		{side: ir.SideMin, text: "lower bound", sentence: "Lower bound", step: []ir.BoundStep{ir.StepValue}, length: true, level: types.List, subject: "the element count of each value of age"},
	} {
		b := ir.BoundConst{Side: c.side, Path: c.step, Length: c.length, Level: &ir.TypeRef{Kind: c.level}}
		if got := ir.BoundSideText(c.side); got != c.text {
			t.Errorf("BoundSideText(%d) = %q, want %q", c.side, got, c.text)
		}
		if got := ir.BoundSideSentence(c.side); got != c.sentence {
			t.Errorf("BoundSideSentence(%d) = %q, want %q", c.side, got, c.sentence)
		}
		if got := ir.BoundSubject(&ir.Field{Name: "age"}, b); got != c.subject {
			t.Errorf("BoundSubject(%+v) = %q, want %q", c, got, c.subject)
		}
	}
}
