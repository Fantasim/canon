package canon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/wire"
	"github.com/fantasim/canonlang/internal/workspace"
)

// API.md X2: a panic inside Apply is an *InternalError with its stack; nothing is written, and
// the project stays usable, for reads and for the next edit.
func TestEditPanicInApply(t *testing.T) {
	src := []byte("/// C.\npackage c\n\n/// N.\nlet n: Int = 1\n")
	renames := 0
	fsys := newWriteFS(map[string][]byte{
		"/law/project.canon": []byte("project a {\n  canon: \"0.1\"\n}\n"),
		"/law/c/c.canon":     src,
	}, func() { renames++ })
	p, err := Open("/law", Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	editHost = func(*build.Analysis) wire.Host { panic("host bug") }
	_, err = p.Edit(context.Background(), Edit{Ops: []Op{Set("c:n", Int(2))}})
	editHost = build.EditHost
	var ie *InternalError
	if !errors.As(err, &ie) || ie.Msg != "host bug" || ie.Stack == "" {
		t.Fatalf("Edit: %v", err)
	}
	if got, _ := fsys.ReadFile("/law/c/c.canon"); !slices.Equal(got, src) || renames != 0 {
		t.Errorf("API.md X2: %d renames, c.canon %q", renames, got)
	}
	if v, err := p.Value(context.Background(), "c:n"); err != nil || v.Text != "1" {
		t.Errorf("API.md X2: Value after the panic: %v", err)
	}
	if _, err := p.Edit(context.Background(), Edit{Ops: []Op{Set("c:n", Int(2))}}); err != nil {
		t.Errorf("API.md X2: Edit after the panic: %v", err)
	}
}

// editRefusals are edit's and workspace's failures and the API error each is (API.md §15, X1).
var editRefusals = []struct {
	err  error
	want error
	text string
}{
	{&edit.OpError{Index: 2, Path: "a:x", Err: &edit.CollisionError{Paths: []string{"a/x.canon", "a/y.canon"}}}, ErrPathCollision, "op 2: a:x: file already exists: a/x.canon, a/y.canon"},
	{&edit.OpError{Index: 0, Path: "a:x", Err: &edit.NotEditableError{Reason: edit.ReasonComputed, Origin: "a:y", Refs: []string{"b.p", "b.q"}}}, ErrNotEditable, "op 0: a:x: value is not editable: computed: b.p, b.q"},
	{&edit.OpError{Index: 1, Path: "a:x", Err: &edit.ValueError{Expected: "Int", Got: "Str(\"x\")"}}, ErrBadValue, "op 1: a:x: value does not fit the type: expected Int, got Str(\"x\")"},
	{&edit.OpError{Index: 1, Path: "a:x", Err: edit.ErrKeyExists}, ErrKeyExists, "op 1: a:x: key already exists"},
	{&edit.OpError{Index: 1, Path: "a:x", Err: fmt.Errorf("wrapped: %w", edit.ErrInternal)}, ErrInternal, "internal compiler error: wrapped: edit failed inside the compiler"},
	{edit.ErrStableKey, ErrStableKey, "stable id cannot be removed, renamed or un-retired"},
	{&edit.StaleError{Files: []string{"a/a.canon"}}, ErrStale, "sources changed since the base revision: a/a.canon"},
	{&workspace.StaleError{Files: []string{"b/b.canon"}}, ErrStale, "sources changed since the base revision: b/b.canon"},
	{&workspace.OverlayError{File: "a/a.canon"}, ErrOverlay, "a/a.canon: file has an unsaved overlay"},
	{&workspace.NotCanonicalError{Files: []string{"c/c.canon"}}, ErrNotCanonical, "file is not in canonical layout: c/c.canon"},
	{fs.ErrPermission, fs.ErrPermission, "permission denied"},
	{fmt.Errorf("a.canon: %w", edit.ErrChanges), ErrInternal, "internal compiler error: a.canon: invalid file changes"},
	{fmt.Errorf("%w: hidden path: .x", edit.ErrUnwritable), ErrProject, "invalid project: file cannot be written by an edit: hidden path: .x"},
}

// API.md X1, API.md X2, API.md E12, API.md N3, API.md S12, API.md M9, API.md N9, API.md S11:
// each failure of an edit is the API's error of its kind, with the text X1 fixes.
func TestEditErrorMapping(t *testing.T) {
	p := &Project{}
	for _, c := range editRefusals {
		got := p.editError(context.Background(), c.err)
		if !errors.Is(got, c.want) || got.Error() != c.text {
			t.Errorf("%v: %v, want %v %q", c.err, got, c.want, c.text)
		}
	}
	var ne *NotEditableError
	if err := p.editError(context.Background(), editRefusals[1].err); !errors.As(err, &ne) || ne.Origin != "a:y" || ne.Reason != ReasonComputed {
		t.Errorf("API.md W5: %+v", ne)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.editError(ctx, edit.ErrJournal); !errors.Is(err, ErrProject) {
		t.Errorf("log-2026-09-29 M4 U5b-r: a journal under a cancelled ctx: %v", err)
	}
	if err := p.editError(ctx, edit.ErrKeyExists); !errors.Is(err, context.Canceled) {
		t.Errorf("API.md S11: %v", err)
	}
}
