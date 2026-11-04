package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// WIRE.md §5.6, CODEGEN.md §5.6 (E8019 InlineFoldedKey): a key of own folds onto a key of all that own does not hold when they are equal but for the case of ASCII letters, the loaders' case rule; non-ASCII letters do not fold.
func TestFoldsOnto(t *testing.T) {
	tests := []struct {
		name     string
		own, all []string
		want     bool
	}{
		{"no keys", nil, nil, false},
		{"the same key is the key", []string{"a"}, []string{"a"}, false},
		{"a key differing by case", []string{"kind"}, []string{"Kind"}, true},
		{"a key of own is never another", []string{"Kind", "kind"}, []string{"Kind", "kind"}, false},
		{"one of two folds", []string{"a", "Kind"}, []string{"a", "kind"}, true},
		{"another length", []string{"kind"}, []string{"kinds"}, false},
		{"a letter beyond ASCII", []string{"é"}, []string{"É"}, false},
		{"the Kelvin sign is no k", []string{"k"}, []string{"K"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ir.FoldsOnto(tt.own, tt.all); got != tt.want {
				t.Errorf("FoldsOnto(%q, %q) = %v, want %v", tt.own, tt.all, got, tt.want)
			}
		})
	}
}
