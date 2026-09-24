package escapes_test

import (
	"fmt"

	d "example.com/features/d/out/go"
	escapes "example.com/features/escapes/out/go"
	q "example.com/features/q/out/go"
)

// Smoke test of parameters and locals named like imported packages.
func Example() {
	fmt.Println(escapes.F(q.QLow), escapes.F(q.QHigh), escapes.GateOf(d.DNorth, q.QHigh).ID(), escapes.GateOf(d.DNorth, q.QLow) == nil)
	west := escapes.GetGates().Get(escapes.GateIDWest)
	fmt.Println(west.Q(), west.D(), escapes.GateOf(d.DSouth, q.QLow) == west)
	// Output:
	// true false east true
	// LOW south true
}
