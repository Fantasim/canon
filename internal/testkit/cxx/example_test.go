package cxx_test

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

const (
	goBinary  = "go"
	goVersion = "version"
	versionIs = "go version"
)

// Example shows the first flag every compile uses; it never invokes a compiler.
func Example() {
	fmt.Println(cxx.Flags[0])
	// Output: -std=c++17
}

func ExampleHold() {
	func() {
		defer cxx.Hold()()
		fmt.Println("holding a slot")
	}()
	// Output: holding a slot
}

func ExampleCombined() {
	out, err := cxx.Combined(exec.Command(goBinary, goVersion))
	fmt.Println(err, strings.HasPrefix(string(out), versionIs))
	// Output: <nil> true
}

func ExampleOutput() {
	out, err := cxx.Output(exec.Command(goBinary, goVersion))
	fmt.Println(err, strings.HasPrefix(string(out), versionIs))
	// Output: <nil> true
}

func ExampleCompile() {
	out, err := cxx.Compile(exec.Command(goBinary, goVersion))
	fmt.Println(err, strings.HasPrefix(string(out), versionIs))
	// Output: <nil> true
}
