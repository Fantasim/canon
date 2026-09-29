package diag

import (
	"reflect"
	"testing"
)

// mutableValue is a Value argument whose text changes after the finding is built.
type mutableValue struct{ text string }

func (v *mutableValue) CanonText() string { return v.text }

// IMPLEMENTATION-PLAN §7.6 NFR-02: a detached builder renders the same finding, holding no argument.
func TestDetachedRendersTheSameFinding(t *testing.T) {
	for i, b := range constructed {
		d := b.Detached()
		if !reflect.DeepEqual(d.finding(sampleFiles, sampleName), b.finding(sampleFiles, sampleName)) {
			t.Errorf("constructed %d (%s): detached renders another finding", i, b.def.Code)
		}
		for j, a := range d.args {
			switch a.(type) {
			case sampleValueArg, sampleTypeArg:
				t.Errorf("constructed %d (%s): argument %d kept by reference", i, b.def.Code, j)
			}
		}
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: a detached builder keeps its values' texts of then, nested too.
func TestDetachedKeepsTextNotValues(t *testing.T) {
	v := &mutableValue{text: `"before"`}
	inner := E3501.At(sampleSpan, v, sampleName)
	b := E1703.AtType(sampleSpan, sampleName, inner.Message()).Stack([]Frame{{Fn: sampleName, Span: sampleSpan}})
	d := b.Detached()
	want := d.finding(sampleFiles, sampleName).Message
	v.text = `"after"`
	if got := d.finding(sampleFiles, sampleName).Message; got != want {
		t.Errorf("detached message changed with its value: %q, want %q", got, want)
	}
	if b.finding(sampleFiles, sampleName).Message == want {
		t.Error("the original builder should render the value's new text")
	}
	b.stack[0].Fn = sampleText
	if d.stack[0].Fn != sampleName {
		t.Error("the detached stack shares the original's")
	}
}
