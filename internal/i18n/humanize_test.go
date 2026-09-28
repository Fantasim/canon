package i18n_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/i18n"
)

// VIEWMODEL.md X1: humanize inserts a space at each lower/upper ASCII boundary, replaces '_'
// with a space, collapses runs of spaces, trims, lower-cases, then upper-cases the first
// character.
func TestHumanize(t *testing.T) {
	tests := []struct{ name, want string }{
		{"farmPurchasePrice", "Farm purchase price"},
		{"minLevel", "Min level"},
		{"id", "Id"},
		{"stage1Rate", "Stage1 rate"},
		{"adventureQuests", "Adventure quests"},
		{"max_level", "Max level"},
	}
	for _, tt := range tests {
		if got := i18n.Humanize(tt.name); got != tt.want {
			t.Errorf("Humanize(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
