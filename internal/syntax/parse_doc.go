package syntax

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// docBlock is a doc block in a token's leading trivia: its first and last doc lines, and
// whether a blank line separates it from what follows.
type docBlock struct {
	first, last int
	blankAfter  bool
}

// docBlocks splits leading trivia into doc blocks: consecutive doc lines with no blank line
// between them; ordinary comments may stand between them.
func docBlocks(lead []Trivia) []docBlock {
	var blocks []docBlock
	open := false
	breaks := 0
	for i, t := range lead {
		switch t.Kind {
		case TriviaNewline:
			breaks++
			if breaks > 1 && open {
				blocks[len(blocks)-1].blankAfter = true
				open = false
			}
		case TriviaDocComment:
			if !open {
				blocks = append(blocks, docBlock{first: i})
				open = true
			}
			blocks[len(blocks)-1].last = i
			breaks = 0
		case TriviaLineComment, TriviaBlockComment:
			breaks = 0
		default:
		}
	}
	return blocks
}

// doc attaches the doc block directly before host, the first token of an attachable item, and
// returns it; nil when there is none.
func (p *parser) doc(host Tok) *DocComment {
	lead := p.toks[host].Leading
	blocks := docBlocks(lead)
	if len(blocks) == 0 || blocks[len(blocks)-1].blankAfter {
		return nil
	}
	b := blocks[len(blocks)-1]
	p.docs[host] = true
	var lines []string
	for _, t := range lead[b.first : b.last+1] {
		if t.Kind == TriviaDocComment {
			lines = append(lines, string(p.src.Content[t.Start:t.End]))
		}
	}
	return &DocComment{
		Bounds: Bounds{From: host, To: host}, Start: lead[b.first].Start, End: lead[b.last].End,
		Text: docText(lines),
	}
}

// sweepDocs reports W1001 for every doc block that attached to nothing (GRAMMAR.md §9.1).
func (p *parser) sweepDocs() {
	for i, t := range p.toks {
		blocks := docBlocks(t.Leading)
		for j, b := range blocks {
			if j == len(blocks)-1 && !b.blankAfter && p.docs[Tok(i)] {
				continue
			}
			sp := source.Span{File: p.src.ID, Start: t.Leading[b.first].Start, End: t.Leading[b.last].End}
			diag.W1001.At(sp).Report(p.bag)
		}
	}
}
