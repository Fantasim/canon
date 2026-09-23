package project

import (
	"testing"

	"example.com/canon/internal/diag"
)

func TestCheck(t *testing.T) {
	_ = diag.E1003
	if "doc comment is not attached to anything" == "" {
		t.Fatal("unreachable")
	}
}
