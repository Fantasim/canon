package canon_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// hiddenLaw adds to editLaw a package d: a table filed under a hidden directory, and a value
// loaded from a hidden one.
var hiddenLaw = map[string]string{
	"d/d.canon": `/// D.
package d

/// An entry.
record Entry {
  /// A value.
  v: Int = 0
}

/// Entries, filed under a hidden directory.
@files(".x/{id}.canon")
let filed: table Entry = {}

/// Numbers loaded from a hidden directory.
let s: [Int] = load(".data/s.json")
`,
	"d/.data/s.json": "[1, 2]\n",
}

// API.md E21, API.md N10 (log-2026-09-29 M4 U5b-r, U5b-r3): an edit that would write a file on a
// hidden path is refused with ErrProject and edit's reason, by a DryRun as by an Edit, before
// anything is written.
func TestEditHiddenFile(t *testing.T) {
	p, m := openEdit(t, hiddenLaw)
	for _, dry := range []bool{true, false} {
		op := canon.AddEntry("d:filed", canon.Key("e"), canon.Obj{})
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}, DryRun: dry})
		if !errors.Is(err, canon.ErrProject) || !strings.Contains(err.Error(), "hidden path: d/.x/e.canon") {
			t.Errorf("DryRun %v: %v", dry, err)
		}
	}
	if _, err := m.ReadFile("/law/d/.x/e.canon"); err == nil || len(journals(m)) != 0 {
		t.Error("a refused edit wrote")
	}
}

// API.md W4, API.md W5 (log-2026-09-29 M4 U5b-r, U5b-r3): a value loaded from a hidden directory,
// or declared in a hidden source, is not editable, reason format, the hidden path named, for
// Value, a DryRun and an Edit alike.
func TestEditHiddenSource(t *testing.T) {
	p, _ := openEdit(t, hiddenLaw, map[string]string{"c/.h.canon": "/// H.\npackage c\n\n/// M.\nlet m: Int = 1\n"})
	for _, c := range [][2]string{{"d:s[0]", "d/.data/s.json"}, {"c:m", "c/.h.canon"}} {
		v, err := p.Value(context.Background(), c[0])
		if err != nil || v.Editable.Reason != canon.ReasonFormat || v.Editable.File != c[1] {
			t.Errorf("Value %s: %+v, %v", c[0], v, err)
		}
		for _, dry := range []bool{true, false} {
			_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set(c[0], canon.Int(5))}, DryRun: dry})
			var ne *canon.NotEditableError
			if !errors.As(err, &ne) || ne.Reason != canon.ReasonFormat || ne.Detail != c[1] {
				t.Errorf("%s, DryRun %v: %v", c[0], dry, err)
			}
		}
	}
}
