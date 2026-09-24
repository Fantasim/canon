package keys_test

import (
	"fmt"

	keys "example.com/features/keys/out/go"
)

// Smoke test of key-only refs.
func Example() {
	for p := range keys.GetPosts().All() {
		prev, hasPrev := p.PreviousID()
		planned, hasPlanned := p.PlannedIDs()
		fmt.Println(p.ID(), p.StatusID(), prev, hasPrev, p.SeenIDs().Clone(), planned.Clone(), hasPlanned, p.Reply() != nil)
	}
	s, ok := keys.StatusOf(keys.PostIdHello)
	_, none := keys.StatusOf(keys.PostIdBye)
	fmt.Println(keys.GetFirstID(), s, ok, none, keys.IsOpen(keys.StatusIdOpen), keys.StatusIdClosed)
	// Output:
	// hello open open false [open closed] [closed] true false
	// bye closed open true [] [] false true
	// open open true false true closed
}
