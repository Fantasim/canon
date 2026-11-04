package canon

import "testing"

// API.md E19, F7, X1: the count is the result's, the cap's dropped errors included; a count
// below the findings kept (a value built by hand) is the findings'.
func TestRejectedErrorText(t *testing.T) {
	tests := []struct {
		err  RejectedError
		want string
	}{
		{RejectedError{Findings: make([]Finding, 1), total: 3}, "edit rejected: it produces errors: 3 errors, 2 dropped by the cap"},
		{RejectedError{Findings: make([]Finding, 2), total: 2}, "edit rejected: it produces errors: 2 errors"},
		{RejectedError{Findings: make([]Finding, 2), total: 1}, "edit rejected: it produces errors: 2 errors"},
		{RejectedError{total: 1}, "edit rejected: it produces errors: 1 error, 1 dropped by the cap"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}
