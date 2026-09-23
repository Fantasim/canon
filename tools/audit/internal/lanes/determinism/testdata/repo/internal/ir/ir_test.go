package ir

import "testing"

func TestRanges(t *testing.T) {
	for k := range map[string]bool{"a": true} {
		t.Log(k)
	}
}
