package i18n_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// A package with one field and no French file: its catalogue holds a key per translatable
// text, and Check reports the missing count for each other project language.
func Example() {
	fs := &source.FileSet{}
	bag := diag.NewBag(fs, "")
	src, err := fs.Add("greeting/greeting.canon", "/greeting/greeting.canon", []byte(`package greeting

/// A short hello.
record Hello {
  /// Who it greets.
  name: String
}

emit view { out: "out/hello.view.json" }
`))
	if err != nil {
		panic(err)
	}
	file := syntax.Parse(src, syntax.FileSource, bag)

	proj := project.New("example", project.Version{Major: 0, Minor: 1})
	proj.Languages = []string{"en", "fr"}

	bags := check.Bags{}
	prog := check.Check(context.Background(), proj, []*syntax.File{file}, bags, eval.NewFolder(bags, eval.Options{}))
	res := i18n.Check(prog, proj, bags, map[string]bool{"greeting": true})

	greeting := res["greeting"]
	var keys []string
	for _, e := range greeting.Catalogue.Entries {
		keys = append(keys, e.Key)
	}
	fmt.Println(keys, "fr missing:", greeting.Languages["fr"].Missing)
	// Output: [Hello.help Hello.name Hello.name.help] fr missing: 3
}
