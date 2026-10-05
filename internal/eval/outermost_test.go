package eval_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
)

// EVALUATION.md §2.3, DECISIONS 324: the precomputation's frame is always outermost, the cut count exact.
func TestOutermost(t *testing.T) {
	frames := func(n int) []diag.Frame {
		out := make([]diag.Frame, n)
		for i := range out {
			out[i] = diag.Frame{Fn: "f" + strconv.Itoa(i)}
		}
		return out
	}
	pc := diag.Frame{Fn: "Hold.bad()"}
	full := frames(diag.MaxStackFrames)
	atCap := append(slices.Clone(full[:diag.MaxStackFrames-1]), pc)
	cases := []struct {
		name     string
		stack    []diag.Frame
		more     int
		f        *diag.Frame
		hidden   bool // f is among the frames cut
		want     []diag.Frame
		wantMore int
	}{
		{"no frame", frames(2), 0, nil, false, frames(2), 0},
		{"empty stack", nil, 0, &pc, false, []diag.Frame{pc}, 0},
		{"added outermost", frames(2), 0, &pc, false, append(frames(2), pc), 0},
		{"already held", append(frames(2), pc), 0, &pc, false, append(frames(2), pc), 0},
		{"at the cap", full, 0, &pc, false, atCap, 1},
		{"past a cut, f not cut", full, 4, &pc, false, atCap, 5},
		{"past a cut, f cut", full, 4, &pc, true, atCap, 4},
		{"cut to nothing, f cut", nil, 3, &pc, true, []diag.Frame{pc}, 2},
		{"cut to nothing, f not cut", nil, 3, &pc, false, []diag.Frame{pc}, 3},
	}
	for _, c := range cases {
		got, more := eval.Outermost(c.stack, c.more, c.f, c.hidden)
		if !slices.Equal(got, c.want) || more != c.wantMore {
			t.Errorf("%s: %v (%d more), want %v (%d more)", c.name, got, more, c.want, c.wantMore)
		}
	}
}
