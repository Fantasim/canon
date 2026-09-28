package viewgen

import (
	"reflect"
	"testing"
)

// dashStruct exercises jsonTag's `json:"-"` skip (encoding/json's own convention); no vm
// struct uses it today, but a future schema field must not appear if it ever does.
type dashStruct struct {
	A string `json:"a"`
	B string `json:"-"`
}

// TestJSONDashSkipped is go.md 2's tag reading: `json:"-"` never appears as a member.
func TestJSONDashSkipped(t *testing.T) {
	n, err := buildValue(reflect.ValueOf(dashStruct{A: "x", B: "y"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Members) != 1 || n.Members[0].Key != "a" {
		t.Errorf("members = %+v, want only \"a\"", n.Members)
	}
}
