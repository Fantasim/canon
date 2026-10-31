package fixture_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fantasim/canonlang/internal/testkit/fixture"
)

// Tiers is the block WithoutTiers removes.
func ExampleTiers() {
	project := "project p {\n  tiers {\n    a: [x]\n  }\n}\n"
	fmt.Printf("%q %q\n", fixture.Tiers(project), fixture.WithoutTiers(project))
	// Output: "  tiers {\n    a: [x]\n  }\n" "project p {\n}\n"
}

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
