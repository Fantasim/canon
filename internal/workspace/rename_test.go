package workspace_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// renameLaw is a project where a function renamed `max` captures a call of the built-in, one
// renamed `cap` captures nothing, and package q imports p.
func renameLaw() map[string]string {
	return map[string]string{
		"/law/project.canon": lawProject,
		"/law/p/p.canon":     "/// P.\npackage p\n\n/// Bigger.\nfn bigger(a: Int, b: Int) -> Int {\n  return a\n}\n\n/// Top.\nlet top: Int = max(1, 2)\n",
		"/law/q/q.canon":     "/// Q.\npackage q\n\nimport p\n\n/// Twice.\nlet twice: Int = p.bigger(a: 1, b: 2) * 2\n",
	}
}

// renameReq renames name to newName.
func renameReq(name, newName string) workspace.EditRequest {
	op := edit.Operation{Kind: edit.OpRenameName, Path: name, Name: newName}
	return workspace.EditRequest{Changes: workspace.Changes{Host: build.EditHost, Ops: []edit.Operation{op}}}
}

// API.md E35, API.md E21: after a clean re-check, an identifier that would name another
// declaration refuses the rename with ErrNameClash naming it; nothing is written, nothing published.
func TestRenameCapture(t *testing.T) {
	fsys := newMemFS(renameLaw())
	p := open(t, fsys)
	published := 0
	defer p.Subscribe(func(workspace.Event) { published++ })()
	_, err := p.Edit(context.Background(), renameReq("p:bigger", "max"))
	var ne *edit.NameError
	if !errors.Is(err, edit.ErrNameClash) || !errors.As(err, &ne) || !strings.HasPrefix(ne.Detail, "p/p.canon:10:16 would name p:max") {
		t.Fatalf("API.md E35: %v", err)
	}
	if data, _ := fsys.ReadFile("/law/p/p.canon"); !strings.Contains(string(data), "fn bigger(") || published != 0 {
		t.Errorf("API.md E35: a refused rename wrote or published")
	}
}

// API.md E33, API.md E17, API.md E37: a rename writes the declaring package and its importer,
// re-checks both, and its Undo is the reverse RenameName.
func TestRenameApplies(t *testing.T) {
	fsys := newMemFS(renameLaw())
	p := open(t, fsys)
	out, err := p.Edit(context.Background(), renameReq("p:bigger", "larger"))
	if err != nil || !out.Applied || out.Checked.Summary.Packages != 2 {
		t.Fatalf("API.md E33: %+v, %v", out, err)
	}
	if data, _ := fsys.ReadFile("/law/q/q.canon"); !strings.Contains(string(data), "p.larger(a: 1, b: 2)") {
		t.Errorf("API.md E33: q is %q", data)
	}
	want := edit.Operation{Kind: edit.OpRenameName, Path: "p:larger", Name: "bigger"}
	if len(out.Plan.Undo) != 1 || out.Plan.Undo[0].Path != want.Path || out.Plan.Undo[0].Name != want.Name {
		t.Errorf("API.md E37: undo %+v", out.Plan.Undo)
	}
}

// API.md E31: the request is judged first: with another op, or AllowErrors, ErrBadOp; under an
// edit layer, reason layer; a draft never holds a RenameName.
func TestRenameRequest(t *testing.T) {
	p := open(t, newMemFS(renameLaw()))
	ctx := context.Background()
	other := edit.Operation{Kind: edit.OpSet, Path: "p:top", Value: edit.Int(3)}
	many := renameReq("p:nothing", "x")
	many.Ops = append(many.Ops, other)
	errs := renameReq("p:nothing", "x")
	errs.AllowErrors = true
	for _, req := range []workspace.EditRequest{many, errs} {
		if _, err := p.Edit(ctx, req); !errors.Is(err, edit.ErrBadOp) {
			t.Errorf("API.md E31: %v", err)
		}
	}
	layer := renameReq("p:nothing", "x")
	layer.EditLayer = "dev"
	var ne *edit.NotEditableError
	if _, err := p.Edit(ctx, layer); !errors.As(err, &ne) || ne.Reason != edit.ReasonLayer {
		t.Errorf("API.md E31: under a layer: %v", err)
	}
	s := read(t, p)
	if _, err := workspace.Draft(ctx, s, analyze(t, s), renameReq("p:bigger", "y").Changes); !errors.Is(err, edit.ErrBadOp) {
		t.Errorf("API.md E31: a draft: %v", err)
	}
}
