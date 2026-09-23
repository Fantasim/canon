package syntax

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// stringNode is a string literal node, usable as every kind of name or value a string can be.
type stringNode interface {
	StrLit
	nameLitNode()
	annValueNode()
	projectValueNode()
}

// strLit is a stringLit: STRING, RAW, MLSTRING or RAWML, the interpolated ones as their pieces
// with the expressions between them; nil when there is none.
func (p *parser) strLit() stringNode {
	switch k := p.kind(); k {
	case TokString, TokMLString:
		t := p.next()
		s := &StringLit{Bounds: Bounds{From: t, To: t}, Multiline: k == TokMLString}
		s.Parts = textParts(p.decodeString([]Tok{t}, k == TokMLString))
		return s
	case TokStringHead, TokMLStringHead:
		if s := p.interpString(); s != nil {
			return s
		}
		return nil
	case TokRaw, TokRawML:
		t := p.next()
		r := &RawStringLit{Bounds: Bounds{From: t, To: t}, Multiline: k == TokRawML}
		r.Value = string(p.rawValue(t))
		return r
	default:
	}
	p.fail(stringName)
	return nil
}

// constString is a stringLit without interpolation (E1132, GRAMMAR.md §2.6).
func (p *parser) constString() stringNode {
	s := p.strLit()
	if lit, ok := s.(*StringLit); ok && hasInterp(lit) {
		diag.E1132.At(p.nodeSpan(lit)).Report(p.bag)
	}
	return s
}

func hasInterp(s *StringLit) bool {
	for _, part := range s.Parts {
		if part.Interp != nil {
			return true
		}
	}
	return false
}

// interpString is a head piece, then { interpolation middle } interpolation tail.
func (p *parser) interpString() *StringLit {
	start := p.pos
	ml := p.kind() == TokMLStringHead
	mid, tail := TokStringMid, TokStringTail
	if ml {
		mid, tail = TokMLStringMid, TokMLStringTail
	}
	pieces := []Tok{p.next()}
	var interps []*Interp
	for {
		interps = append(interps, p.interp())
		switch p.kind() {
		case mid:
			pieces = append(pieces, p.next())
			continue
		case tail:
			pieces = append(pieces, p.next())
		default:
			p.brokenPiece()
			return nil
		}
		break
	}
	s := &StringLit{Bounds: p.from(start), Multiline: ml}
	for i, text := range p.decodeString(pieces, ml) {
		if text != "" {
			s.Parts = append(s.Parts, StringPart{Text: text})
		}
		if i < len(interps) {
			s.Parts = append(s.Parts, StringPart{Interp: interps[i]})
		}
	}
	return s
}

// brokenPiece fails an interpolation not followed by the rest of its string: at a line break
// or the end of the file the lexer has reported it (E1112), elsewhere it is E1116.
func (p *parser) brokenPiece() {
	if p.at(TokEOF) || p.at(TokNL) || !p.sameLine(p.pos-1, p.pos) {
		p.bail = true
		return
	}
	p.fail(expected(TokRBrace))
}

// interp is an interpolation's expression and format spec; an empty one (E1112, reported by
// the lexer) holds an empty BadExpr.
func (p *parser) interp() *Interp {
	start := p.pos
	it := &Interp{}
	if k := p.kind(); isPiece[k] || k == TokFormatSpec {
		it.X = p.badExpr(start)
	} else {
		it.X = p.expr()
	}
	if p.at(TokFormatSpec) {
		t := p.next()
		fs, _ := parseSpec(p.src.Content[p.toks[t].Start+1 : p.toks[t].End])
		fs.Tok = t
		it.Spec = &fs
	}
	it.Bounds = p.from(start)
	return it
}

func textParts(texts []string) []StringPart {
	var parts []StringPart
	for _, t := range texts {
		if t != "" {
			parts = append(parts, StringPart{Text: t})
		}
	}
	return parts
}

// decodeString is the text of each piece of a string, escapes decoded and, for a multiline
// string, its layout applied (LEX-03).
func (p *parser) decodeString(pieces []Tok, ml bool) []string {
	raw := make([][]byte, len(pieces))
	for i, t := range pieces {
		raw[i] = p.pieceBody(t, ml)
	}
	if ml {
		raw = layout(raw, p.closingPrefix(pieces[len(pieces)-1]))
	}
	out := make([]string, len(raw))
	for i, r := range raw {
		out[i] = unescape(r)
	}
	return out
}

// pieceBody is the text of a string piece between its delimiters: a quote or `"""` and its
// line, "{" and "}"; a multiline string's closing line is left out.
func (p *parser) pieceBody(t Tok, ml bool) []byte {
	b := p.src.Content[p.toks[t].Start:p.toks[t].End]
	open, closeQuote := quoteText, quoteText
	if ml {
		open, closeQuote = tripleQuote, tripleQuote
	}
	if cut, ok := bytes.CutPrefix(b, []byte(open)); ok {
		b = cut
		if ml {
			b = afterOpeningLine(b)
		}
	} else {
		b = bytes.TrimPrefix(b, []byte(closeBraceText))
	}
	if cut, ok := bytes.CutSuffix(b, []byte(closeQuote)); ok {
		b = cut
		if ml {
			b = beforeClosingLine(b)
		}
	} else {
		b = bytes.TrimSuffix(b, []byte(openBraceText))
	}
	return b
}

// rawValue is the value of a raw string: its text between the delimiters, a multiline one with
// its layout applied.
func (p *parser) rawValue(t Tok) []byte {
	b := p.src.Content[p.toks[t].Start+source.Pos(len(rawPrefix)) : p.toks[t].End]
	if p.toks[t].Kind == TokRaw {
		return bytes.TrimSuffix(bytes.TrimPrefix(b, []byte(quoteText)), []byte(quoteText))
	}
	body := afterOpeningLine(bytes.TrimPrefix(b, []byte(tripleQuote)))
	if cut, ok := bytes.CutSuffix(body, []byte(tripleQuote)); ok {
		body = beforeClosingLine(cut)
	}
	return layout([][]byte{body}, p.closingPrefix(t))[0]
}

// closingPrefix is the indentation before the closing `"""` of the piece t.
func (p *parser) closingPrefix(t Tok) []byte {
	b := p.src.Content[p.toks[t].Start:p.toks[t].End]
	b = bytes.TrimSuffix(b, []byte(tripleQuote))
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return b[i+1:]
	}
	return nil
}

// afterOpeningLine drops the blanks and the line break after an opening `"""`.
func afterOpeningLine(b []byte) []byte {
	rest := bytes.TrimLeft(b, blankChars)
	if len(rest) > 0 && rest[0] == '\n' {
		return rest[1:]
	}
	return rest
}

// beforeClosingLine drops the last line break and the indentation before a closing `"""`.
func beforeClosingLine(b []byte) []byte {
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return b[:i]
	}
	return b[:0]
}

// layout removes the indentation prefix from the start of each line of a multiline string;
// a blank line no longer than the prefix becomes empty.
func layout(pieces [][]byte, prefix []byte) [][]byte {
	out := make([][]byte, len(pieces))
	lineStart := true
	for i, piece := range pieces {
		out[i] = stripLines(piece, prefix, lineStart)
		lineStart = len(piece) > 0 && piece[len(piece)-1] == '\n'
	}
	return out
}

// stripLines removes the prefix from each line of piece; the first line only at lineStart.
func stripLines(piece, prefix []byte, lineStart bool) []byte {
	var b []byte
	for len(piece) > 0 {
		if lineStart {
			piece = stripIndent(piece, prefix)
		}
		line, rest, found := bytes.Cut(piece, []byte(lf))
		b = append(b, line...)
		if found {
			b = append(b, '\n')
		}
		piece, lineStart = rest, true
	}
	return b
}

func stripIndent(line, prefix []byte) []byte {
	end := bytes.IndexByte(line, '\n')
	if end < 0 {
		end = len(line)
	}
	if isBlank(line[:end]) && end <= len(prefix) {
		return line[end:]
	}
	return bytes.TrimPrefix(line, prefix)
}
