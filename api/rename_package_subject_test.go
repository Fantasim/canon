package canon_test

import (
	"context"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// packageSubjectSrc holds a let, a package check over it and a test with two substitutions of it.
const packageSubjectSrc = "/// C.\npackage c\n\n/// The count.\nlet n: Int = 1\n\n" +
	"check small: n < 5 else \"too big: {n}\"\n\n" +
	"test \"substitutions\" {\n  expect package { n: 2 } passes\n  expect package { n: 7 } fails small\n}\n"

// API.md E33, API.md E37, EVALUATION.md §10.5: a rename rewrites substitution targets; tests pass; the Undo restores.
func TestRenameSubstitutionTarget(t *testing.T) {
	files := map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "c/c.canon": packageSubjectSrc}
	opts := project(files)
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	ctx := context.Background()
	if res, err := p.Test(ctx, canon.TestOptions{}); err != nil || res.Passed != 1 || res.Failed != 0 {
		t.Fatalf("API.md E33, before the rename: %v %+v", err, res)
	}
	res, err := renameOnce(p, "c:n", "m")
	if err != nil {
		t.Fatalf("API.md E33: %v", err)
	}
	after := read(t, m, "c/c.canon")
	for _, want := range []string{"let m: Int = 1", "check small: m < 5 else \"too big: {m}\"",
		"expect package { m: 2 } passes", "expect package { m: 7 } fails small"} {
		if !strings.Contains(after, want) {
			t.Errorf("API.md E33: no %q in\n%s", want, after)
		}
	}
	if tr, err := p.Test(ctx, canon.TestOptions{}); err != nil || tr.Passed != 1 || tr.Failed != 0 {
		t.Errorf("API.md E33, after the rename: %v %+v", err, tr)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E37, undo: %v", err)
	}
	if got := read(t, m, "c/c.canon"); got != packageSubjectSrc {
		t.Errorf("API.md E37: the undo gives %q", got)
	}
}
