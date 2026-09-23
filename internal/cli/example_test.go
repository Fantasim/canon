package cli_test

import (
	"context"
	"fmt"
	"os"

	"github.com/fantasim/canonlang/internal/cli"
)

// Main runs one command line and returns its exit code (CLI.md §2.5).
func Example() {
	env := cli.Env{Stdout: os.Stdout, Stderr: os.Stdout, Dir: "."}
	code := cli.Main(context.Background(), []string{"version", "--format", "json"}, env)
	fmt.Println("exit", code)
	// Output:
	// {"version":{"compiler":"0.1.0","languages":["0.1"],"fingerprint":"canon-fp v1","viewModel":"canon-vm/1","lock":"canon.lock v1","commit":""}}
	// exit 0
}
