package lookups_test

import (
	"fmt"
	"testing"

	lookups "example.com/data/demo/lookups/out/go"
)

// Smoke test of a lookup method's key getters, resolved and key only.
func TestKeyGetters(t *testing.T) {
	st := lookups.GetStatuses()
	a, b := st.Get(lookups.StatusIDA), st.Get(lookups.StatusIDB)
	id, ok := a.NextID(true)
	_, none := a.NextID(false)
	if a.Next(true) != b || a.Next(false) != nil || id != lookups.StatusIDB || !ok || none {
		t.Error("next")
	}
	if fmt.Sprint(a.PeersIDs(true).Clone()) != "[b a]" || a.PeersIDs(false).Len() != 0 || a.Peers(true).At(0) != b {
		t.Error("peers")
	}
	if b.OtherID(false) != lookups.OtherIDX {
		t.Error("other")
	}
}
