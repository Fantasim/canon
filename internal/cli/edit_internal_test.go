package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// CLI.md §3.15, API.md R6: a poisoned value's summary, and one with no findings, exit 1.
func TestEditPoisoned(t *testing.T) {
	findings := []canon.Finding{{Severity: canon.SeverityError, Package: "a"}, {Severity: canon.SeverityError, Package: "b"}, {Severity: canon.SeverityWarning, Package: "a"}}
	if s := poisonedSummary(findings); s.Packages != 2 || s.Errors != 2 || s.Warnings != 1 {
		t.Errorf("poisonedSummary = %+v", s)
	}
	var out, errs bytes.Buffer
	o := newOptions()
	o.format = formatJSON
	inv := &invocation{ctx: context.Background(), env: Env{Stdout: &out, Stderr: &errs}, opt: o}
	code := inv.editFail(&canon.PathError{Op: -1, Path: "a:n", Err: canon.ErrNoValue}, time.Now())
	if code != exitErrors || out.Len() != 0 || !strings.Contains(errs.String(), canon.ErrNoValue.Error()) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errs.String())
	}
}
