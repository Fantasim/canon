package format

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// note is one comment as the printer places it (FORMATTER.md §8).
type note struct {
	text     string
	blank    bool // a blank line precedes it in the input
	sameLine bool // it follows another comment on that comment's line
	joined   bool // the token it leads follows it on its line
	doc      bool // a doc comment line as the lexer read it (GRAMMAR.md §2.2)
}

// attached is what the printer keeps of a token's trivia: its own-line comments, its trailing
// comments, and whether a blank line directly precedes it.
type attached struct {
	lead  []note
	trail []note
	blank bool
}

// attach reads every token's comments, placed as decision 168 says.
func attach(f *syntax.File, drop []bool) []attached {
	out := make([]attached, len(f.Tokens))
	var pending []note
	prev, carry := 0, false // carry: a blank line before a dropped token, kept for what follows it
	for i, t := range f.Tokens {
		if t.Kind == syntax.TokNL || t.Kind == syntax.TokBOF {
			continue
		}
		lead, blank := leading(f, t.Leading)
		trail, moved := trailing(f, t.Trailing)
		if drop[i] {
			left := append(dropped(lead, trail, startsLine(t)), moved...)
			carry = gapAt(left, 0, carry)
			carry = gapAt(left, len(lead), blank) || carry
			if !startsLine(t) {
				out[prev].trail = append(out[prev].trail, trail...)
			}
			pending = append(pending, left...)
			continue
		}
		carry = gapAt(lead, 0, carry)
		out[i] = attached{lead: append(pending, lead...), trail: trail, blank: blank || carry}
		pending, prev, carry = moved, i, false
	}
	return out
}

// gapAt puts a blank line before notes[at], or reports it left for what follows the notes.
func gapAt(notes []note, at int, blank bool) bool {
	if !blank || at >= len(notes) {
		return blank
	}
	notes[at].blank = true
	return false
}

// dropped are the own-line comments a dropped token leaves to the next token: its leading
// ones, and its trailing ones when it starts a line, on the line of the last leading one.
func dropped(lead, trail []note, own bool) []note {
	out := slices.Clone(lead)
	if len(out) > 0 {
		out[len(out)-1].joined = false
	}
	if !own {
		return out
	}
	for i, n := range trail {
		n.sameLine = i > 0 || len(lead) > 0 && lead[len(lead)-1].joined
		out = append(out, n)
	}
	return out
}

// startsLine reports a token whose leading trivia holds a line break.
func startsLine(t syntax.Token) bool {
	return slices.ContainsFunc(t.Leading, func(tr syntax.Trivia) bool { return tr.Kind == syntax.TriviaNewline })
}

// leading reads the comments of a token's leading trivia, and whether a blank line separates
// the last of them (or the previous token) from the token; a last comment on the token's line
// is joined to it.
func leading(f *syntax.File, tr []syntax.Trivia) ([]note, bool) {
	var notes []note
	breaks := 0
	for _, t := range tr {
		switch t.Kind {
		case syntax.TriviaNewline:
			breaks++
		case syntax.TriviaLineComment, syntax.TriviaDocComment, syntax.TriviaBlockComment:
			n := note{text: commentText(f, t), blank: breaks >= blankRun, doc: t.Kind == syntax.TriviaDocComment}
			n.sameLine = breaks == 0 && len(notes) > 0
			notes = append(notes, n)
			breaks = 0
		default:
		}
	}
	if n := len(notes); n > 0 && breaks == 0 && !strings.Contains(notes[n-1].text, newlineText) {
		notes[n-1].joined = true
	}
	return notes, breaks >= blankRun
}

// trailing reads the comments on a token's line, and splits off the ones from a block comment
// spanning lines on, which lead the next token.
func trailing(f *syntax.File, tr []syntax.Trivia) (kept, moved []note) {
	for _, t := range tr {
		if t.Kind != syntax.TriviaLineComment && t.Kind != syntax.TriviaBlockComment {
			continue
		}
		s := commentText(f, t)
		switch {
		case len(moved) > 0:
			moved = append(moved, note{text: s, sameLine: true})
		case strings.Contains(s, newlineText):
			moved = append(moved, note{text: s})
		default:
			kept = append(kept, note{text: s})
		}
	}
	return kept, moved
}

// commentText is a comment without trailing whitespace on any of its lines, a doc line as
// "/// text".
func commentText(f *syntax.File, t syntax.Trivia) string {
	s := string(f.Src.Content[t.Start:t.End])
	if t.Kind == syntax.TriviaBlockComment {
		lines := strings.Split(s, newlineText)
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, trailingBlanks)
		}
		return strings.Join(lines, newlineText)
	}
	s = strings.TrimRight(s, trailingBlanks)
	if t.Kind != syntax.TriviaDocComment {
		return s
	}
	rest := strings.TrimPrefix(s, docLead)
	if rest != "" && !strings.HasPrefix(rest, space) {
		return docLead + space + rest
	}
	return s
}

// isLine reports a line comment, which ends its line.
func isLine(n note) bool { return !strings.HasPrefix(n.text, blockOpen) }

// apart reports each note whose blank line before it separates two doc blocks (GRAMMAR.md §2.2).
func apart(notes []note) []bool {
	out := make([]bool, len(notes))
	open := false
	for i, n := range notes {
		if n.blank {
			out[i], open = open && slices.ContainsFunc(notes[i:], isDoc), false
		}
		open = open || n.doc
	}
	return out
}

// isDoc reports a doc comment line.
func isDoc(n note) bool { return n.doc }
