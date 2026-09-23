package syntax

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Parse parses src, the project's project.canon when kind is FileProject and any other .canon
// file otherwise; every finding goes to bag, and it returns a whole tree even after errors.
func Parse(src *source.File, kind FileKind, bag *diag.Bag) *File {
	p := &parser{src: src, bag: bag, docs: map[Tok]bool{}}
	p.toks = separate(src.Content, lex(src, bag))
	p.pos = NoTok + 1
	f := p.file(kind == FileProject)
	p.sweepDocs()
	return f
}

// bodyCtx is what encloses a statement: a function, a check block, a test block, loops.
type bodyCtx struct {
	fn, check, test bool
	loops           int
}

// parser is a recursive descent over the separated tokens. bail is set by the first syntax
// error of an item and cleared where the parser resumes: until then no other E1116 is
// reported and lists stop.
type parser struct {
	src     *source.File
	bag     *diag.Bag
	toks    []Token
	pos     Tok
	bail    bool
	header  bool
	regexOK bool
	regex   *RegexLit
	depth   int
	ctx     bodyCtx
	docs    map[Tok]bool
}

func (p *parser) kind() TokenKind { return p.toks[p.pos].Kind }

func (p *parser) peek(n int) TokenKind {
	if i := int(p.pos) + n; i < len(p.toks) {
		return p.toks[i].Kind
	}
	return TokEOF
}

func (p *parser) at(k TokenKind) bool { return p.kind() == k }

// next consumes the current token and returns it; EOF is never consumed.
func (p *parser) next() Tok {
	t := p.pos
	if p.kind() != TokEOF {
		p.pos++
	}
	return t
}

func (p *parser) accept(k TokenKind) Tok {
	if p.at(k) {
		return p.next()
	}
	return NoTok
}

// expect consumes a k, or reports E1116 and returns NoTok.
func (p *parser) expect(k TokenKind) Tok {
	if p.at(k) {
		return p.next()
	}
	p.fail(expected(k))
	return NoTok
}

// expected names a token kind for E1116: a class by its name, a fixed token quoted.
func expected(k TokenKind) string {
	if k < TokEllipsis {
		return k.String()
	}
	return quoted(k.String())
}

func quoted(word string) string { return quoteText + word + quoteText }

// startsItem reports a token that can start a list item: the parser reads it as the next item
// (E1117) rather than as junk after the previous one.
func startsItem(k TokenKind) bool {
	return startsItemTok[k] || (isWord(k) && !continuesLine[k] && k != KwAs)
}

// fail reports E1116 at the current token, unless the item already has one, and bails.
func (p *parser) fail(expected ...string) { p.failAt(p.pos, expected...) }

// failAt is fail at token t; an ILLEGAL token is not reported again (the lexer reported it).
func (p *parser) failAt(t Tok, expected ...string) {
	if !p.bail && p.toks[t].Kind != TokIllegal {
		sp := p.span(t, t)
		if p.toks[t].Kind == TokEOF {
			diag.E1116.AtEof(sp, expected).Report(p.bag)
		} else {
			diag.E1116.AtToken(sp, expected, p.found(t)).Report(p.bag)
		}
	}
	p.bail = true
}

// found is a token as E1116 quotes it: its first line, or its class when it has no text.
func (p *parser) found(t Tok) string {
	text := p.src.Content[p.toks[t].Start:p.toks[t].End]
	if i := bytes.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	if len(text) == 0 {
		return p.toks[t].Kind.String()
	}
	return string(text)
}

func (p *parser) span(from, to Tok) source.Span {
	return source.Span{File: p.src.ID, Start: p.toks[from].Start, End: p.toks[to].End}
}

func (p *parser) nodeSpan(n Node) source.Span { return p.span(n.First(), n.Last()) }

func (p *parser) text(t Tok) string {
	return string(p.src.Content[p.toks[t].Start:p.toks[t].End])
}

// atWord reports the contextual keyword w at the current token (GRAMMAR.md §4.2).
func (p *parser) atWord(w string) bool { return p.wordAt(p.pos, w) }

func (p *parser) wordAt(t Tok, w string) bool {
	tok := p.toks[t]
	return tok.Kind == TokIdent && string(p.src.Content[tok.Start:tok.End]) == w
}

// from bounds a node from start to the last token consumed; a node that consumed none (it only
// holds an empty Bad node) is empty too.
func (p *parser) from(start Tok) Bounds { return Bounds{From: start, To: p.pos - 1} }

// badBounds bounds a Bad node: the tokens skipped since start or, when none was, nothing: an
// empty node at the unexpected token (Last = First-1), so it never overlaps a neighbour.
func (p *parser) badBounds(start Tok) Bounds {
	return Bounds{From: min(start, p.pos), To: p.pos - 1}
}

func (p *parser) badExpr(start Tok) *BadExpr { return &BadExpr{Bounds: p.badBounds(start)} }

// bounded is every node: its bounds can be moved once its last token is known.
type bounded interface {
	Node
	reset(b Bounds)
}

func (b *Bounds) reset(nb Bounds) { *b = nb }

// enter counts one more level of nesting and refuses past maxNesting (E1116), so that no input
// exhausts the stack; leave undoes it.
func (p *parser) enter() bool {
	if p.depth >= maxNesting {
		p.fail(nestingName)
		p.skipGroup()
		return false
	}
	p.depth++
	return true
}

func (p *parser) leave() { p.depth-- }

func (p *parser) sameLine(a, b Tok) bool {
	return bytes.IndexByte(p.src.Content[p.toks[a].End:p.toks[b].Start], '\n') < 0
}

// adjacent reports that token b starts where token a ends, with nothing between them.
func (p *parser) adjacent(a, b Tok) bool { return p.toks[a].End == p.toks[b].Start }

// skipNL consumes the NL tokens GRAMMAR.md allows before a continuation ({ NL }).
func (p *parser) skipNL() {
	for p.at(TokNL) {
		p.next()
	}
}

// ident consumes an IDENT, reporting E1125 for a reserved word naming a kind of declaration
// and E1116 for anything else, in which case it returns nil.
func (p *parser) ident(kind diag.Kind) *Ident {
	k := p.kind()
	if k >= KwAnd && k < TokenKindCount {
		diag.E1125.At(p.span(p.pos, p.pos), p.text(p.pos), kind).Report(p.bag)
	} else if k != TokIdent {
		p.fail(identName)
		return nil
	}
	t := p.next()
	return &Ident{Bounds: Bounds{From: t, To: t}, Name: p.text(t)}
}

// ref consumes an IDENT naming something declared elsewhere, or reports E1116.
func (p *parser) ref() *Ident {
	if !p.at(TokIdent) {
		p.fail(identName)
		return nil
	}
	t := p.next()
	return &Ident{Bounds: Bounds{From: t, To: t}, Name: p.text(t)}
}

// word consumes a WORD: an IDENT or any reserved word (GRAMMAR.md §2.3, §4.3).
func (p *parser) word() *Ident {
	if !isWord(p.kind()) {
		p.fail(wordName)
		return nil
	}
	t := p.next()
	return &Ident{Bounds: Bounds{From: t, To: t}, Name: p.text(t)}
}

// dataWord is a WORD naming a data symbol, where none, true, false and self are E1126.
func (p *parser) dataWord() *Ident {
	if k := p.kind(); k == KwNone || k == KwTrue || k == KwFalse || k == KwSelf {
		diag.E1126.At(p.span(p.pos, p.pos), p.text(p.pos)).Report(p.bag)
	}
	return p.word()
}

// qualified reads one { "." one }, going on while cont accepts the token after a ".": a
// qualifiedWord with word and isWord, a qualifiedIdent with ref and isIdent.
func (p *parser) qualified(one func() *Ident, cont func(TokenKind) bool) *QualifiedName {
	start := p.pos
	q := &QualifiedName{}
	for {
		part := one()
		if part == nil {
			return nil
		}
		q.Parts = append(q.Parts, part)
		if !p.at(TokDot) || !cont(p.peek(1)) {
			break
		}
		p.next()
	}
	q.Bounds = p.from(start)
	return q
}

func (p *parser) qualifiedWord() *QualifiedName { return p.qualified(p.word, isWord) }

func (p *parser) qualifiedIdent() *QualifiedName { return p.qualified(p.ref, isIdent) }

func isIdent(k TokenKind) bool { return k == TokIdent }
