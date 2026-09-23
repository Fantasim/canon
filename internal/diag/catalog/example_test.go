package catalog_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag/catalog"
)

// Packages reads the package table of the implementation plan, in dependency order.
func ExamplePackages() {
	plan := "| Package (under `internal/` unless noted) | Owns | Implements | Consumes |\n" +
		"|---|---|---|---|\n" +
		"| `source` | positions | API.md | — |\n" +
		"| `eval` | interpreter; `eval/std` stdlib | EVALUATION.md | source |\n" +
		"| `api/` (package `canon`) | public API | API.md | eval |\n"
	pkgs, err := catalog.Packages([]byte(plan))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, p := range pkgs {
		fmt.Print(p.Name, " ", p.Dir, p.Subs, "; ")
	}
	// Output: source internal/source[]; eval internal/eval[internal/eval/std]; api api[];
}
