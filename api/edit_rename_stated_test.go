package canon_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// statedCase is a bound ref stated in each instance, first's naming its own entry with second's
// key: src has {k} where second's entry and its ref state the renamed key.
type statedCase struct {
	name, src, path, old string
}

// statedCases are a level-1 ref on A naming its oo's element, and a level-2 ref on S naming its inn's.
var statedCases = []statedCase{
	{"level 1", "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\n" +
		"record A {\n  id: String\n  oo: [S] keyed by k\n  pick: ref S\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }, { k: \"y\" }], pick: \"x\" },\n" +
		"  { id: \"second\", oo: [{ k: \"{k}\" }], pick: \"{k}\" },\n]\n", "p:aa[second].oo[x]", "x"},
	{"level 2", "package p\n\nrecord T {\n  k: String\n  w: Int = 0\n}\n\n" +
		"record S {\n  k: String\n  inn: [T] keyed by k\n  pick: ref T\n}\n\n" +
		"record A {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\", inn: [{ k: \"q\" }, { k: \"y\" }], pick: \"q\" }] },\n" +
		"  { id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"{k}\" }, { k: \"r\" }], pick: \"{k}\" }] },\n]\n", "p:aa[second].oo[x].inn[q]", "q"},
}

// statedRefs is each ref's kind and line, in order.
func statedRefs(refs []canon.Ref) string {
	var out []string
	for _, r := range refs {
		out = append(out, fmt.Sprintf("%s %d", r.Kind, r.Line))
	}
	return strings.Join(out, ", ")
}

// API.md R7, E11, E22 (DECISIONS 410, 473): a key token stating a ref is a value ref of the entry
// it names, judged by identity, never a code ref: renaming second's entry rewrites second's own
// ref and leaves first's, which names first's entry of the same key; the Undo restores the file.
func TestEditRenameLeavesOtherInstanceStatedRef(t *testing.T) {
	for _, c := range statedCases {
		t.Run(c.name, func(t *testing.T) {
			src, want := strings.ReplaceAll(c.src, "{k}", c.old), strings.ReplaceAll(c.src, "{k}", "y")
			p, m := openEdit(t, srcLaw(src))
			ctx := context.Background()
			refs, err := p.Refs(ctx, c.path)
			if err != nil {
				t.Fatalf("API.md R7: Refs: %v", err)
			}
			second := fmt.Sprintf("value %d", lineOf(src, "  { id: \"second\""))
			if got := statedRefs(refs.Refs); got != second {
				t.Errorf("API.md R7: the refs of %s are %q, want %q (first's ref names first's entry)", c.path, got, second)
			}
			res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(c.path, canon.Key("y"))}})
			if err != nil {
				t.Fatalf("API.md E19: the rename is refused: %v", err)
			}
			if got := read(t, m, srcMain); got != want {
				t.Errorf("API.md E11, R7: %s after the rename:\n%s\nwant:\n%s", srcMain, got, want)
			}
			if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
				t.Fatalf("API.md E22: the Undo: %v", err)
			}
			if got := read(t, m, srcMain); got != src {
				t.Errorf("API.md E22: %s after the Undo:\n%s", srcMain, got)
			}
		})
	}
}
