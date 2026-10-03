package fixture_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fantasim/canonlang/internal/testkit/fixture"
)

// The copy holds the project file and the example's sources, on disk.
func ExampleCopyRenames() {
	dir, err := os.MkdirTemp("", "fixture")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(dir)
	if err = fixture.CopyRenames(dir, "../../../examples"); err != nil {
		fmt.Println(err)
		return
	}
	_, err = os.Stat(filepath.Join(dir, "project.canon"))
	fmt.Println(err)
	// Output: <nil>
}
