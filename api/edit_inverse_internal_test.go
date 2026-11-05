package canon

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md X1, R5, E22: the Undo check's refusal of an inverse is the user operation's, its index and
// path as given, while the segment its detail names is counted on the inverse's own path.
func TestEditInverseSegment(t *testing.T) {
	p := &Project{}
	err := p.editError(context.Background(), &edit.OpError{Index: 1, Path: "d:x", Inverse: "d:x.a.b.c",
		Err: &edit.PathError{Seg: 2, Err: edit.ErrNoPath}}, "")
	var pe *PathError
	if !errors.As(err, &pe) || !errors.Is(err, ErrNoPath) || pe.Op != 1 || pe.Path != "d:x" || pe.Detail != "segment .c" {
		t.Errorf("an inverse's ErrNoPath at its segment 2: %+v, want op 1, path d:x, detail %q", err, "segment .c")
	}
}
