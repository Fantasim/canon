package diag_test

import (
	"fmt"
	"os"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Each code is a variable whose Def is its registry entry, generated from spec/ERRORS.md.
func Example() {
	def := diag.E3501.Def()
	v := def.Variants[0]
	fmt.Println(def.Code, def.Severity, def.Package, v.Args[0].Type, v.Args[1].Type, v.Template)
	// Output: E3501 error verify Value Name unknown key {key} in {coll}
}

// A package reports into its bag; Render writes the text form (API.md §4.4) or the JSON form.
func ExampleRender() {
	files := diag.MemFiles{{Path: "teamboard/canon.lock", Content: "# canon.lock v1\ntable  teamboard.x\n"}}
	bag := diag.NewBag(files, "teamboard")
	diag.E6005.AtSyntax(source.Span{File: 1, Start: 16, End: 34}).Report(bag)
	opt := diag.RenderOptions{Format: diag.FormatJSON, Summary: bag.Summary()}
	if err := diag.Render(os.Stdout, files, bag.Findings(), opt); err != nil {
		fmt.Println(err)
	}
	// Output: {"severity":"error","code":"E6005","file":"teamboard/canon.lock","line":2,"col":1,"endLine":2,"endCol":19,"package":"teamboard","message":"canon.lock: cannot parse line"}
	// {"summary":{"errors":1,"warnings":0,"packages":1,"ms":0}}
}
