package format

import (
	"bytes"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// builder turns the tree of one file into a layout document. Every token it prints goes
// through word, which prints its comments; a node's first and last tokens are held while
// the node is built, so their comments go outside the node's groups.
type builder struct {
	f         *syntax.File
	notes     []attached
	heldLead  []bool
	heldTrail []bool
	glued     map[*syntax.FieldDecl]bool
	drop      []bool
	leadSum   []int
	trailSum  []int
	idx       *index
	fresh     func(syntax.Tok) bool
	comments  []span // every comment of the file, by offset, once cuts needs them
	focus     *focus // set: the items wholly outside its bytes are left unbuilt
}

func newBuilder(f *syntax.File) *builder {
	drop, kept := make([]bool, len(f.Tokens)), keptCommas(f)
	for i, t := range f.Tokens {
		drop[i] = t.Kind == syntax.TokComma && !kept[i]
	}
	if f.Project != nil {
		for _, e := range f.Project.Items {
			keep := commented(f.Tokens[e.Colon]) || commented(f.Tokens[e.Value.First()])
			drop[e.Colon] = drop[e.Colon] || sugared(e) && !keep
		}
	}
	dropRedundantNones(f, drop)
	b := &builder{
		f: f, notes: attach(f, drop), glued: map[*syntax.FieldDecl]bool{}, drop: drop,
		heldLead: make([]bool, len(f.Tokens)), heldTrail: make([]bool, len(f.Tokens)),
		leadSum: make([]int, len(f.Tokens)+1), trailSum: make([]int, len(f.Tokens)+1),
		idx: newIndex(), fresh: func(syntax.Tok) bool { return false },
	}
	for i, a := range b.notes {
		b.leadSum[i+1], b.trailSum[i+1] = b.leadSum[i]+len(a.lead), b.trailSum[i]+len(a.trail)
	}
	return b
}

// inner reports a comment between the brackets open and close (§6.1).
func (b *builder) inner(open, close syntax.Tok) bool {
	return b.trailSum[close] > b.trailSum[open] || b.leadSum[close+1] > b.leadSum[open+1]
}

// commented reports a comment in a token's trivia.
func commented(t syntax.Token) bool {
	return slices.ContainsFunc(slices.Concat(t.Leading, t.Trailing), isComment)
}

// isComment reports a comment trivia: a line, block or doc comment.
func isComment(tr syntax.Trivia) bool {
	return tr.Kind == syntax.TriviaLineComment || tr.Kind == syntax.TriviaBlockComment || tr.Kind == syntax.TriviaDocComment
}

// raw is the text of token t as written.
func (b *builder) raw(t syntax.Tok) string {
	tk := b.f.Tokens[t]
	return string(b.f.Src.Content[tk.Start:tk.End])
}

// tok is token t as written, with its comments.
func (b *builder) tok(t syntax.Tok) *doc { return b.word(t, b.raw(t)) }

// word is token t spelled s, with its own-line comments before and trailing ones after.
// Inside a construct the comments start a continuation line, one level deeper.
func (b *builder) word(t syntax.Tok, s string) *doc {
	lead := b.lead(t, false)
	if lead != nil {
		lead = indent(lead)
	}
	return cat(lead, text(s), b.trail(t))
}

// leads reports own-line comments before t that the node starting at t prints.
func (b *builder) leads(t syntax.Tok) bool { return !b.heldLead[t] && len(b.notes[t].lead) > 0 }

// closer is a closing bracket without its own-line comments, which its list prints inside.
func (b *builder) closer(t syntax.Tok) *doc { return cat(text(b.raw(t)), b.trail(t)) }

// lead is the own-line comments of t; blanks keeps the blank lines between them and before the
// token, as between the items of a brace list. A blank line between two doc blocks is always
// kept (DECISIONS 211).
func (b *builder) lead(t syntax.Tok, blanks bool) *doc {
	a := b.notes[t]
	if b.heldLead[t] || len(a.lead) == 0 {
		return nil
	}
	keep := apart(a.lead)
	ds := make([]*doc, 0, len(a.lead)+1)
	for i, n := range a.lead {
		ds = append(ds, comment(n, i > 0 && n.blank && (blanks || keep[i])))
	}
	if blanks && a.blank && len(a.lead) > 0 {
		ds = append(ds, blankDoc)
	}
	return cat(ds...)
}

// trail is the trailing comments of t: block comments inline, a line comment at the end of
// the line; then a kept comma after t (DECISIONS 216).
func (b *builder) trail(t syntax.Tok) *doc { return b.trailOf(t, false) }

// trailOf is trail, blanks keeping the kept comma's blank lines as in a brace list.
func (b *builder) trailOf(t syntax.Tok, blanks bool) *doc {
	if b.heldTrail[t] {
		return nil
	}
	var ds []*doc
	notes := b.notes[t].trail
	for i, n := range notes {
		switch {
		case isLine(n):
			ds = append(ds, suffix(n.text))
		case i == len(notes)-1:
			ds = append(ds, text(space+n.text), spaceDoc)
		default:
			ds = append(ds, text(space+n.text))
		}
	}
	return cat(append(ds, b.kept(t, blanks))...)
}

// gap reports a blank line in the input before the item starting at t.
func (b *builder) gap(t syntax.Tok) bool {
	a := b.notes[t]
	if len(a.lead) > 0 {
		return a.lead[0].blank
	}
	return a.blank
}

// node is n with its comments, the ones at its edges outside its own groups.
func (b *builder) node(n syntax.Node) *doc {
	body, trail := b.parts(n, false)
	return cat(body, trail)
}

// item is n as an item of a brace list or of the file, the blank lines among its leading
// comments kept.
func (b *builder) item(n syntax.Node) *doc {
	body, trail := b.parts(n, true)
	return cat(body, trail)
}

// parts is n with its leading comments, and apart its trailing ones and a kept comma after it.
func (b *builder) parts(n syntax.Node, blanks bool) (body, trail *doc) {
	first, last := n.First(), n.Last()
	lead, trail := b.lead(first, blanks), b.trailOf(last, blanks)
	return cat(lead, b.bare(n)), trail
}

// bare is n without the comments at its edges, which stay put when n is re-printed alone.
func (b *builder) bare(n syntax.Node) *doc {
	first, last := n.First(), n.Last()
	wasLead, wasTrail := b.heldLead[first], b.heldTrail[last]
	b.heldLead[first], b.heldTrail[last] = true, true
	body := buildTable[n.Kind()](b, n)
	b.heldLead[first], b.heldTrail[last] = wasLead, wasTrail
	return body
}

// after is the first token after t that is not a separator NL; before, the last one before.
func (b *builder) after(t syntax.Tok) syntax.Tok {
	for t++; b.f.Tokens[t].Kind == syntax.TokNL; t++ {
	}
	return t
}

func (b *builder) before(t syntax.Tok) syntax.Tok {
	for t--; b.f.Tokens[t].Kind == syntax.TokNL; t-- {
	}
	return t
}

// oneLine reports whether tokens from and to start on the same line of the input (§6.1).
func (b *builder) oneLine(from, to syntax.Tok) bool {
	src := b.f.Src.Content
	return !bytes.Contains(src[b.f.Tokens[from].Start:b.f.Tokens[to].Start], []byte(newlineText))
}
