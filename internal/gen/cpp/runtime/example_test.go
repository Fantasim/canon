package cppruntime_test

import (
	"fmt"
	"slices"

	cppruntime "github.com/fantasim/canonlang/internal/gen/cpp/runtime"
)

// The identifiers of the runtime headers include the flat map and the guard of canon_runtime.h.
func ExampleIdentifiers() {
	ids := cppruntime.Identifiers()
	fmt.Println(slices.Contains(ids, "FlatMap"), slices.Contains(ids, "CANON_RUNTIME_RT_V1"), slices.Contains(ids, "NOT_THERE"))
	// Output: true true false
}
