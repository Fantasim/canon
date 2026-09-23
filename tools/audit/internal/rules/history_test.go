package rules

import "testing"

func TestNarratesHistory(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"HistoryNote used to return an error here", true},
		{"this parser used to SKIP a non-positive id", true},
		{"this field was renamed in the schema", true},
		{"the value was renamed in the schema", false},
		{"fixed in v2.1", true},
		{"fixed in commit abc123", true},
		{"a bug fixed in the parser", false},
		{"rows previously seen are skipped", false},
		{"the previously-expanded node stays open", false},
		{"previously this returned nil", true},
		{"Used to resolve a define name", false},
		{"the token is used to sign requests", false},
		{"a case a real request can no longer reach", false},
		{"the legacy layout still loads", false},
	}
	for _, tt := range tests {
		if got := NarratesHistory(tt.text); got != tt.want {
			t.Errorf("NarratesHistory(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestNarratesHistoryQuoted(t *testing.T) {
	if NarratesHistory(`the passive "is used to <verb>" is purpose`) {
		t.Fatal("a quoted passive is not history")
	}
}
