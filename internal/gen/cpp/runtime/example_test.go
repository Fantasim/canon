package cppruntime_test

import (
	"fmt"
	"slices"
	"strings"

	cppruntime "github.com/fantasim/canonlang/internal/gen/cpp/runtime"
)

// The identifiers of the runtime headers include the flat map and the guard of canon_runtime.h.
func ExampleIdentifiers() {
	ids := cppruntime.Identifiers()
	fmt.Println(slices.Contains(ids, "FlatMap"), slices.Contains(ids, "CANON_RUNTIME_RT_V1"), slices.Contains(ids, "NOT_THERE"))
	// Output: true true false
}

// The stamp is 12 hexadecimal digits, and canon_runtime.h as written names its inline namespace by it.
func ExampleStamp() {
	fmt.Println(len(cppruntime.Stamp), strings.Contains(cppruntime.Header, "inline namespace rt_v1_"+cppruntime.Stamp))
	// Output: 12 true
}
