package edit_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// An operation reads and writes the JSON form of an edit (API.md E24, E25, E26).
func ExampleOperation_UnmarshalJSON() {
	var op edit.Operation
	err := json.Unmarshal([]byte(`{"op": "addEntry", "path": "statuses", "key": "blocked", "source": "{ terminal: false }"}`), &op)
	fmt.Printf("%v %s %#v %#v\n", err, op.Path, op.Key, op.Value)
	b, err := json.Marshal(op)
	fmt.Println(string(b), err)
	// Output:
	// <nil> statuses "blocked" "{ terminal: false }"
	// {"op":"addEntry","path":"statuses","key":"blocked","source":"{ terminal: false }"} <nil>
}

// A value is typed as the type expected where it goes (API.md V1, V3).
func ExampleTyper_Value() {
	n := &types.Field{Name: "n", Wire: "n", WirePath: []string{"n"}, Type: types.IntType}
	label := &types.Field{Name: "label", Index: 1, Wire: "label", WirePath: []string{"label"}, Type: types.StringType, Default: &syntax.StringLit{}}
	row := &types.RecordType{Pkg: "p", Name: "Row", Fields: []*types.Field{n, label}}
	ctx := context.Background()
	for _, lit := range []edit.Lit{edit.FromJSON(`{"n": 4}`), edit.Obj{"n": edit.Str("3")}} {
		v, err := edit.Typer{Host: noHost{}}.Value(ctx, lit, row)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(v.CanonText())
	}
	// Output:
	// Row{n: 4, label: }
	// value does not fit the type: expected Int, got Str("3"): at .n
}

// A Change is one file an edit writes; a renamed file keeps its old path.
func ExampleChange() {
	kinds := map[edit.ChangeKind]string{
		edit.ChangeModified: "modified",
		edit.ChangeCreated:  "created",
		edit.ChangeDeleted:  "deleted",
		edit.ChangeRenamed:  "renamed",
	}
	c := edit.Change{Kind: edit.ChangeRenamed, Path: "items/b.canon", OldPath: "items/a.canon"}
	fmt.Println(kinds[c.Kind], c.OldPath, "->", c.Path)
	// Output: renamed items/a.canon -> items/b.canon
}
