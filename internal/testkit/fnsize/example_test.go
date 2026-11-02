package fnsize_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/fnsize"
)

// A generated file is measured as text; the budget is the caller's.
func ExampleCpp() {
	src := "void f() {\n  a();\n  b();\n}\n"
	funcs, err := fnsize.Cpp(src)
	fmt.Println(funcs, err)
	// Output: [{f 2 2}] <nil>
}

func ExampleGo() {
	funcs, err := fnsize.Go("package p\n\nfunc F() []int {\n\treturn []int{\n\t\t1,\n\t}\n}\n")
	fmt.Println(funcs, err)
	// Output: [{F 3 1}] <nil>
}

func ExampleTS() {
	funcs, err := fnsize.TS("const f = (x: number): number => {\n  return x;\n};\nconst g = (x: number) => x;\n")
	fmt.Println(funcs, err)
	// Output: [{f 1 1}] <nil>
}
