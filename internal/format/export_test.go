package format

import (
	"bytes"
	"reflect"
	"weak"

	"github.com/fantasim/canonlang/internal/syntax"
)

// RewriteWhole is Rewrite judged on the whole file at every step, its layout judged afresh:
// what Rewrite around the changed bytes must equal byte for byte.
func RewriteWhole(f *syntax.File, changes []Change) ([]byte, error) {
	// FORMATTER.md §13
	if err := usable(f); err != nil {
		return nil, err
	}
	b := newBuilder(f)
	canonical := bytes.Equal(render(b.file()), f.Src.Content)
	return rewriteWith(b, changes, canonical, false)
}

// SectionsAlone checks every section of f, a fixed point, that prints alone: printed alone it is
// its own bytes. It returns how many it checked and the offsets of those that are not.
func SectionsAlone(f *syntax.File) (int, []int) {
	b := newBuilder(f)
	b.focusOn(span{0, len(f.Src.Content)})
	root := b.file()
	checked, bad := 0, []int(nil)
	for _, n := range b.idx.items {
		p, ok := b.place(root, n)
		if !ok {
			continue
		}
		checked++
		if !bytes.Equal(renderSection(p.d, p.ind), f.Src.Content[p.bytes.lo:p.bytes.hi]) {
			bad = append(bad, p.bytes.lo)
		}
	}
	return checked, bad
}

// SettledAround reports whether the settle judges content, f's text with changes made, on one
// section rather than on the whole file.
func SettledAround(f *syntax.File, content []byte) bool {
	g, err := reparse(f, content)
	if err != nil {
		return false
	}
	_, ok := (&aroundOf{f: f, fb: newBuilder(f)}).judge(g, content)
	return ok
}

// Judged reports whether f's layout is in the cache, judged by an earlier call.
func Judged(f *syntax.File) bool {
	layouts.mu.Lock()
	defer layouts.mu.Unlock()
	_, ok := layouts.files[weak.Make(f)]
	return ok
}

// KeptCommas reports, per token of f, a comma DECISIONS 216 keeps.
func KeptCommas(f *syntax.File) []bool { return keptCommas(f) }

// CommentHosts is, per comment's first byte, the token the formatter's attachment gives it to
// (log-2026-09-29 M4 U1r).
func CommentHosts(f *syntax.File) map[int]syntax.Tok {
	b := newBuilder(f)
	out := map[int]syntax.Tok{}
	for t := range b.notes {
		for _, n := range b.hostedBy(syntax.Tok(t)) {
			out[n.src.lo] = syntax.Tok(t)
		}
	}
	return out
}

// LayoutFields is how many things a file's layout holds: what Adopt must reproduce.
func LayoutFields() int { return reflect.TypeFor[layout]().NumField() }
