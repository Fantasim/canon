package canon_test

import (
	"context"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md E11, E22: pins rename (evaluated refs, not Info): an element's map-literal key and amend `.k` follow it.
func TestEditRenameRewritesNameKeys(t *testing.T) {
	src := "package p\n\nrecord Item {\n  id: String\n  weight: Int = 1\n}\n\n" +
		"let items: [Item] keyed by id = [{ id: \"sword\" }, { id: \"shield\" }]\n\n" +
		"let weights: {ref items: Int} = { shield: 2, sword: 1 }\n\nlet one: ref items = shield\n"
	layer := "package p\nlayer dev\n\namend items {\n  shield.weight: 5\n}\n"
	law := srcLaw(src, "p/dev.layer.canon", layer)
	ctx := context.Background()
	p, m := openEdit(t, law)
	res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename("p:items[shield]", canon.Key("buckler"))}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		file string
		want []string
	}{
		{srcMain, []string{`{ id: "buckler" }`, "{ buckler: 2, sword: 1 }", "= buckler\n"}},
		{"p/dev.layer.canon", []string{"  buckler.weight: 5\n"}},
	} {
		if got := read(t, m, tc.file); !containsAll(got, tc.want) {
			t.Errorf("%s after the rename, want %q:\n%s", tc.file, tc.want, got)
		}
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E23: the Undo: %v", err)
	}
	if got := read(t, m, srcMain); got != src {
		t.Errorf("API.md E22: %s after the Undo:\n%s", srcMain, got)
	}
	if got := read(t, m, "p/dev.layer.canon"); got != layer {
		t.Errorf("API.md E22: the layer after the Undo:\n%s", got)
	}
}

// containsAll reports every want in s.
func containsAll(s string, want []string) bool {
	for _, w := range want {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}
