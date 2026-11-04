package config_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

// Slots reads SlotsEnv as a positive integer; anything else is unset.
func TestSlots(t *testing.T) {
	tests := []struct {
		name, value string
		want        int
		wantOK      bool
	}{
		{"unset", "", 0, false},
		{"positive", "3", 3, true},
		{"zero", "0", 0, false},
		{"negative", "-2", 0, false},
		{"text", "many", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.SlotsEnv, tt.value)
			got, ok := config.Slots()
			if ok != tt.wantOK || (ok && got != tt.want) {
				t.Errorf("Slots() = %d, %v; want %d, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
