package load_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"golang.org/x/tools/txtar"
)

// Example decodes `load.dir("data/*.json")` into a table keyed by each file's stem (WIRE.md §6.5).
func Example() {
	a := &txtar.Archive{Files: []txtar.File{
		{Name: "data/a.json", Data: []byte(`{"id": "a", "name": "A"}`)},
		{Name: "data/b.json", Data: []byte(`{"id": "b", "name": "B"}`)},
	}}
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	layout, _ := project.NewLayout(&project.Project{}, projectDir, nil, bag)
	l := &load.Loader{FS: newMemFS(a), Layout: layout, Set: set}
	v, _, _ := l.Load(context.Background(), load.Request{Pkg: "p", Bag: bag}, dirExpr("data/*.json"), &types.TableType{Elem: itemType()})
	fmt.Println(v.CanonText())
	// Output: {a: Item{id: "a", name: "A"}, b: Item{id: "b", name: "B"}}
}
