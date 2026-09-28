package syntax

import "github.com/fantasim/canonlang/internal/diag"

// braceList parses "{" [ item { SepRun item } [ SepRun ] ] "}". item parses one item and
// reports success; after a failed item the parser skips to the next separator and bad receives
// the bounds of what it skipped.
func (p *parser) braceList(item func() bool, bad func(Bounds)) Delims {
	open := p.expect(TokLBrace)
	if open == NoTok {
		return Delims{}
	}
	header := p.header
	p.header = false
	defer func() { p.header = header }()
	for !p.at(TokRBrace) && !p.at(TokEOF) {
		if p.at(TokComma) || p.at(TokNL) {
			p.fail(itemName)
			p.next()
			p.bail = false
			continue
		}
		start := p.pos
		ok := item()
		if !p.afterItem(start, ok, bad) {
			return Delims{Open: open}
		}
	}
	return Delims{Open: open, Close: p.expect(TokRBrace)}
}

// afterItem checks what follows an item: a separator run, "}" or, on the same line, another
// item (E1117). After an error it resumes at the next separator; it reports false when the
// list cannot go on (the end of the file or a top-level declaration at the start of a line).
func (p *parser) afterItem(start Tok, ok bool, bad func(Bounds)) bool {
	if ok && !p.bail {
		switch {
		case p.at(TokRBrace):
			return true
		case p.at(TokNL) || p.at(TokComma):
			p.sepRun()
			return true
		case startsItem(p.kind()):
			if !p.coalesceEndedLine() {
				diag.E1117.At(p.span(p.pos, p.pos)).Report(p.bag)
			}
			return true
		}
		p.fail(expected(TokNL), expected(TokComma), expected(TokRBrace))
	}
	hard := p.syncList()
	if !ok && bad != nil {
		bad(p.from(start))
	}
	if hard {
		return false
	}
	p.bail = false
	p.sepRun()
	return true
}

// coalesceEndedLine reports a real line break after a "??" that ended an item's type (GRAMMAR.md §3.1 rule 2).
func (p *parser) coalesceEndedLine() bool {
	prev := p.toks[p.pos-1]
	return prev.Kind == TokCoalesce && lineBreak(p.src.Content, prev, p.toks[p.pos])
}

// sepRun consumes a separator run: NL and "," tokens with at most one "," (E1116 for ",,").
func (p *parser) sepRun() {
	p.accept(TokNL)
	if p.accept(TokComma) != NoTok && p.at(TokComma) {
		p.fail(itemName)
	}
	p.accept(TokNL)
}

// syncList skips to the next NL, "," or "}" of the current list; it reports true when it stops
// at the end of the file or at a top-level declaration keyword at the start of a line instead.
func (p *parser) syncList() bool {
	depth := 0
	for {
		k := p.kind()
		switch {
		case k == TokEOF || p.topSync():
			return true
		case depth == 0 && (k == TokNL || k == TokComma || k == TokRBrace):
			return false
		case k == TokLParen || k == TokLBrack || k == TokLBrace:
			depth++
		case (k == TokRParen || k == TokRBrack || k == TokRBrace) && depth > 0:
			depth--
		}
		p.next()
	}
}

// syncTop skips to the next top-level item: after an NL outside brackets, or at a top-level
// declaration keyword at the start of a line.
func (p *parser) syncTop(start Tok) {
	if p.pos == start && !p.at(TokEOF) && !p.at(TokNL) {
		p.next()
	}
	depth := 0
	for !p.at(TokEOF) && !p.topSync() && (depth > 0 || !p.at(TokNL)) {
		switch p.kind() {
		case TokLParen, TokLBrack, TokLBrace:
			depth++
		case TokRParen, TokRBrack, TokRBrace:
			depth = max(depth-1, 0)
		default:
		}
		p.next()
	}
}

// skipGroup skips the current token, or the whole bracketed group it opens.
func (p *parser) skipGroup() {
	depth := 0
	for !p.at(TokEOF) {
		switch p.kind() {
		case TokLParen, TokLBrack, TokLBrace:
			depth++
		case TokRParen, TokRBrack, TokRBrace:
			depth--
		default:
		}
		p.next()
		if depth <= 0 {
			return
		}
	}
}

// topSync reports a token that starts a top-level item at the start of a line, where the parser
// resumes whatever brackets are open.
func (p *parser) topSync() bool {
	t := p.toks[p.pos]
	return topSyncKind[t.Kind] && (t.Start == 0 || p.src.Content[t.Start-1] == '\n')
}

// parenList parses "(" [ item { "," item } [ "," ] ] ")", or with the given brackets; item parses
// one item. It returns the brackets, Close NoTok when they do not close.
func (p *parser) parenList(open, closer TokenKind, item func()) Delims {
	o := p.expect(open)
	if o == NoTok {
		return Delims{}
	}
	header := p.header
	p.header = false
	defer func() { p.header = header }()
	for !p.at(closer) && !p.bail {
		item()
		if p.accept(TokComma) == NoTok {
			break
		}
	}
	if p.at(closer) {
		return Delims{Open: o, Close: p.next()}
	}
	p.fail(expected(TokComma), expected(closer))
	return Delims{Open: o}
}
