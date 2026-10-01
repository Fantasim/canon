package convert_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/convert"
	"github.com/fantasim/canonlang/internal/project"
)

// AdoptOut adds the converted file's path to a list out as a further copy (CLI.md §3.10).
func Example() {
	p := project.New("acme", project.Version{Major: 0, Minor: 1})
	p.Roots = []project.Root{{Name: "game", Path: "../game"}, {Name: "web", Path: "../web"}}
	out, err := convert.AdoptOut(p, "items", []string{"@web/items.json"}, true, "@game/items.json")
	fmt.Println(out, err)
	// Output:
	// [@web/items.json @game/items.json] <nil>
}
