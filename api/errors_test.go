package canon_test

import (
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md X1: every error type writes one form, "<op N: ><path: ><sentinel text><: detail>",
// so its text holds the text of the sentinel errors.Is matches.
func TestErrorTextForm(t *testing.T) {
	located := canon.Finding{Span: canon.Span{File: "project.canon", Line: 2, Col: 5}, Message: "unknown project key \"x\""}
	unlocated := canon.Finding{Message: "root @resource does not exist"}
	tests := []struct {
		err      error
		sentinel error
		want     string
	}{
		{&canon.PathError{Op: -1, Path: "a.b", Err: canon.ErrNoPath}, canon.ErrNoPath, "a.b: no value at path"},
		{&canon.PathError{Op: 3, Err: canon.ErrBadOp, Detail: "d"}, canon.ErrBadOp, "op 3: operation not valid here: d"},
		{&canon.ValueError{Op: -1, Path: "x", Expected: "Int", Got: "String", Detail: "d"}, canon.ErrBadValue, "x: value does not fit the type: expected Int, got String: d"},
		{&canon.NotEditableError{Op: -1, Path: "x", Reason: canon.ReasonPseudo}, canon.ErrNotEditable, "x: value is not editable: pseudo"},
		{&canon.StaleError{Files: []string{"a.canon", "b c.canon"}}, canon.ErrStale, "sources changed since the base revision: a.canon, b c.canon"},
		{&canon.StaleError{}, canon.ErrStale, "sources changed since the base revision"},
		{&canon.RejectedError{Findings: make([]canon.Finding, 1)}, canon.ErrRejected, "edit rejected: it produces errors: 1 error"},
		{&canon.RejectedError{}, canon.ErrRejected, "edit rejected: it produces errors: 0 errors"},
		{&canon.NotCanonicalError{Files: []string{"a.json", "b.json"}}, canon.ErrNotCanonical, "file is not in canonical layout: a.json, b.json"},
		{&canon.ProjectError{Err: canon.ErrProject, Findings: []canon.Finding{located, unlocated}}, canon.ErrProject, "invalid project: project.canon:2:5: unknown project key \"x\""},
		{&canon.ProjectError{Err: canon.ErrUnsupportedVersion}, canon.ErrUnsupportedVersion, "unsupported language version"},
		{&canon.SyntaxError{Findings: []canon.Finding{unlocated}}, canon.ErrSyntax, "syntax error: root @resource does not exist"},
		{&canon.SyntaxError{}, canon.ErrSyntax, "syntax error"},
		{&canon.InternalError{Msg: "boom"}, canon.ErrInternal, "internal compiler error: boom"},
	}
	for _, tt := range tests {
		got := tt.err.Error()
		if got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
		if !errors.Is(tt.err, tt.sentinel) || !strings.Contains(got, tt.sentinel.Error()) {
			t.Errorf("%q does not carry its sentinel %q", got, tt.sentinel)
		}
	}
}
