package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// staleLayer is a layer dev of package a amending a status the base does not have.
var staleLayer = map[string]string{"a/dev.layer.canon": "package a\nlayer dev\n\namend statuses {\n  gone.label: \"Gone\"\n}\n"}

// DECISIONS 463, API.md E18, E19, E20: an edit that plans lock lines while an inactive layer is
// broken judges the layer on the snapshot its own re-check read: the layer's finding alone, named
// by it, refused without AllowErrors, written with it, its lock line appended.
func TestEditLockLinesInactiveLayer(t *testing.T) {
	code := diag.E1905.Def().Code
	for _, allow := range []bool{false, true} {
		p, m := openEdit(t, staleLayer)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{addSecond}, AllowErrors: allow})
		var findings []canon.Finding
		var re *canon.RejectedError
		switch {
		case allow && (err != nil || !res.Applied):
			t.Fatalf("AllowErrors: %v, want the edit written", err)
		case allow:
			findings = res.Findings
		case !errors.As(err, &re):
			t.Fatalf("%v, want the edit refused by its re-check", err)
		default:
			findings = re.Findings
		}
		if _, ok := layerFinding(findings, code, "dev"); !ok || len(findings) != 1 {
			t.Errorf("AllowErrors %v: %v, want dev's %s alone", allow, findings, code)
		}
		want := lockFirst
		if allow {
			want += "table  a.codes  second\n"
		}
		if got := lockOf(t, m, "a/canon.lock"); got != want {
			t.Errorf("AllowErrors %v: lock %q, want %q", allow, got, want)
		}
	}
}
