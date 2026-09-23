package golden_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// A golden case is a txtar archive: its inputs, then the expected output in want.
func ExampleLoad() {
	cases, err := golden.Load("testdata/*.txtar")
	if err != nil {
		fmt.Println(err)
		return
	}
	want, err := cases[0].Want()
	fmt.Printf("%s %d files, want %q %v\n", cases[0].Path, len(cases[0].Archive.Files), want, err)
	// Output: testdata/trivial.txtar 2 files, want "canon\n" <nil>
}
