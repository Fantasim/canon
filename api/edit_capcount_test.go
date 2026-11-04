package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// capCountCases are edits refused for errors under a cap of one finding (API.md E19, F7).
var capCountCases = []struct {
	name   string
	layers []string
	more   string
	op     canon.Op
	want   string // RejectedError.Error()
	kept   int    // error findings the cap kept
	trunc  []canon.Truncation
}{
	{
		name: "the cap drops the only error, no layer involved",
		more: "\nwarn k0 > 100 else \"low\"\n", layers: []string{"lw"},
		op:   canon.Set("d:qs[a].items", canon.Bool(false)),
		want: "edit rejected: it produces errors: 1 error, 1 dropped by the cap", kept: 0,
		trunc: []canon.Truncation{{Package: "d", Errors: 1}},
	},
	{
		name: "the cap drops an inactive layer's error",
		more: "\nwarn k0 > 100 else \"low\"\n",
		op:   canon.Remove("d:ps[one]"),
		want: "edit rejected: it produces errors: 1 error, 1 dropped by the cap", kept: 0,
		trunc: []canon.Truncation{{Package: "d", Errors: 1}},
	},
	{
		name: "the cap keeps the error",
		op:   canon.Remove("d:ps[one]"),
		want: "edit rejected: it produces errors: 1 error", kept: 1,
	},
}

// API.md F7, E19: the refusal counts the errors from Summary.Errors, the dropped ones included,
// and says how many the cap dropped; the findings are those F7 keeps.
func TestEditRejectedCountsDropped(t *testing.T) {
	for _, c := range capCountCases {
		res, err := allLayersOpenMax(t, c.layers, c.more, 1).Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
		var re *canon.RejectedError
		if !errors.As(err, &re) || res == nil || res.Applied {
			t.Errorf("%s: %v, want the edit rejected", c.name, err)
			continue
		}
		if got := re.Error(); got != c.want {
			t.Errorf("%s: Error() = %q, want %q", c.name, got, c.want)
		}
		if len(re.Findings) != c.kept || res.Summary.Errors != 1 || !slices.Equal(res.Summary.Truncated, c.trunc) {
			t.Errorf("%s: findings %v, summary %+v: want %d error kept, 1 counted, truncated %v", c.name, re.Findings, res.Summary, c.kept, c.trunc)
		}
	}
}
