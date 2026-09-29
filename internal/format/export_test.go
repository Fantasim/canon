package format

import "github.com/fantasim/canonlang/internal/syntax"

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
