package edit_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// Apply computes an edit in memory: the files it would write and the operations that undo it.
// Nothing is written; a refusal names the operation.
func ExampleApply() {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/p/p.canon":     file("package p\n\n/// Rows.\nlet rows: {String: Int} = { \"a\": 1 }\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(context.Background(), []string{"p"})
	if err != nil {
		fmt.Println(err)
		return
	}
	env := edit.Env{Project: p, Host: hostOf}
	add := edit.Operation{Kind: edit.OpAddEntry, Path: "rows", Key: edit.Key("b"), Value: edit.Int(2)}
	plan, err := edit.Apply(context.Background(), env, edit.NewSnapshot(a), edit.Request{Ops: []edit.Operation{add}})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%s %q\n", plan.Changes[0].Path, plan.Changes[0].After)
	add.Key = edit.Key("a")
	_, err = edit.Apply(context.Background(), env, edit.NewSnapshot(a), edit.Request{Ops: []edit.Operation{add}})
	var oe *edit.OpError
	fmt.Println(plan.Undo[0].Path, plan.Touched, errors.As(err, &oe), oe.Index, errors.Is(err, edit.ErrKeyExists))
	// Output: p/p.canon "package p\n\n/// Rows.\nlet rows: {String: Int} = { \"a\": 1, \"b\": 2 }\n"
	// p:rows[b] [p] true 0 true
}

// The refusals of an edit are sentinels the API maps to its own (API.md §15).
func ExampleNotEditableError() {
	refusals := []error{
		&edit.NotEditableError{Reason: edit.ReasonKey}, edit.ErrKeyExists, edit.ErrStableKey,
		edit.ErrPathCollision, edit.ErrNoProject,
	}
	var notEditable []bool
	for _, err := range refusals {
		notEditable = append(notEditable, errors.Is(err, edit.ErrNotEditable))
	}
	fmt.Println(notEditable)
	// Output: [true false false false false]
}
