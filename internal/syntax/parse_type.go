package syntax

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
)

// typeTable dispatches a primType on its first token (GRAMMAR.md §5.9, DECISIONS 26).
var typeTable [TokenKindCount]func(*parser) Type

// pastTable is typeTable after a contextual `past`: no "(" or "{", a second `past` a name (GRAMMAR.md §4.2).
var pastTable [TokenKindCount]func(*parser) Type

func init() {
	typeTable[TokIdent], typeTable[TokLBrack], typeTable[TokLBrace] = (*parser).identType, (*parser).listType, (*parser).mapType
	typeTable[KwStable], typeTable[KwTable], typeTable[KwRef] = (*parser).tableType, (*parser).tableType, (*parser).refType
	typeTable[KwFn], typeTable[KwAsset], typeTable[KwMatch] = (*parser).fnType, (*parser).assetType, (*parser).matchType
	typeTable[TokUnderscore], typeTable[TokLParen] = (*parser).anyType, (*parser).parenType
	for k := range typeTable {
		if startsString[k] {
			typeTable[k] = (*parser).literalType
		}
	}
	pastTable = typeTable
	pastTable[TokIdent], pastTable[TokLParen], pastTable[TokLBrace] = (*parser).namedType, nil, nil
}

// typ is unionType = optType { "|" optType } (GRAMMAR.md §5.9).
func (p *parser) typ() Type {
	start := p.pos
	if !p.enter() {
		return &BadType{Bounds: p.badBounds(start)}
	}
	defer p.leave()
	t := p.optType()
	if !p.at(TokPipe) {
		return t
	}
	u := &UnionType{Alts: []Type{t}}
	for p.accept(TokPipe) != NoTok {
		u.Alts = append(u.Alts, p.optType())
	}
	u.Bounds = p.from(start)
	return u
}

// optType is primType [ "?" ] [ "where" expr ]; "T??" is lexed "??" and read as two "?"
// (E3401 belongs to check).
func (p *parser) optType() Type {
	start := p.pos
	t := p.primType()
	switch {
	case p.at(TokQuestion):
		p.next()
		t = &OptionalType{Bounds: p.from(start), Elem: t}
	case p.at(TokCoalesce):
		p.next()
		t = &OptionalType{Bounds: p.from(start), Elem: &OptionalType{Bounds: p.from(start), Elem: t}}
	}
	t = p.keyedBy(start, t)
	if p.accept(KwWhere) != NoTok {
		t = &WhereType{Base: t, Pred: p.expr(), Bounds: p.from(start)}
	}
	return t
}

// primType reads one primary type and its suffixes: typeArgs (once, E1103) and "keyed by".
func (p *parser) primType() Type {
	start := p.pos
	f := typeTable[p.kind()]
	if f == nil {
		p.fail(typeName)
		if !stopToken[p.kind()] && !p.topSync() {
			p.next()
		}
		return &BadType{Bounds: p.badBounds(start)}
	}
	t := f(p)
	args := argsOf(t)
	if args == nil || !p.at(TokLParen) {
		return p.keyedBy(start, t)
	}
	if *args = p.typeArgs(t); *args == nil {
		return &BadType{Bounds: p.badBounds(start)}
	}
	t.(bounded).reset(p.from(start))
	if p.at(TokLParen) {
		second := p.pos
		if p.typeArgs(t) == nil {
			return &BadType{Bounds: p.badBounds(start)}
		}
		diag.E1103.At(p.span(second, p.pos-1)).Report(p.bag)
	}
	return p.keyedBy(start, t)
}

// argsOf is where the typeArgs of t go: a named, list, map or dependent map type.
func argsOf(t Type) **TypeArgs {
	switch t := t.(type) {
	case *NamedType:
		return &t.Args
	case *ListType:
		return &t.Args
	case *MapType:
		return &t.Args
	case *DepMapType:
		return &t.Args
	}
	return nil
}

// keyedBy reads "keyed" "by" IDENT after a type: a list type, else E1137 (GRAMMAR.md §5.9).
func (p *parser) keyedBy(start Tok, t Type) Type {
	if !p.atWord(wordKeyed) || !p.wordAt(p.pos+1, wordBy) {
		return t
	}
	if _, ok := t.(*ListType); !ok {
		diag.E1137.At(p.span(p.pos, p.pos+1)).Report(p.bag)
	}
	p.next()
	p.next()
	key := p.ref()
	if key == nil {
		return &BadType{Bounds: p.badBounds(start)}
	}
	return &KeyedType{List: t, Key: key, Bounds: p.from(start)}
}

// typeArgs is "(" typeArg { "," typeArg } [ "," ] ")", a typeArg being REGEX or expr; a REGEX
// is allowed only as the only argument of a named type.
func (p *parser) typeArgs(t Type) *TypeArgs {
	start := p.pos
	_, named := t.(*NamedType)
	a := &TypeArgs{}
	p.regexOK, p.regex = named && p.peek(1) == TokRegex, nil
	d := p.parenList(TokLParen, TokRParen, func() { a.Args = append(a.Args, p.expr()) })
	p.regexOK = false
	if d.Close == NoTok {
		return nil
	}
	p.loneRegex(a.Args)
	a.Bounds = p.from(start)
	return a
}

// loneRegex reports E1115 for the REGEX let through at the start of args unless it is the only
// argument, whole.
func (p *parser) loneRegex(args []Expr) {
	r := p.regex
	p.regex = nil
	if r != nil && (len(args) != 1 || args[0] != Expr(r)) {
		diag.E1115.At(p.nodeSpan(r)).Report(p.bag)
	}
}

// identType is a namedType, or a pastType when `past` is a keyword (GRAMMAR.md §4.2).
func (p *parser) identType() Type {
	if !p.pastKeyword() {
		return p.namedType()
	}
	start := p.next()
	t := pastTable[p.kind()](p)
	if pt, ok := t.(pastable); ok {
		*pt.pastSlot() = start
		t.(bounded).reset(p.from(start))
		return t
	}
	t.(bounded).reset(p.badBounds(start))
	return t
}

// pastKeyword reports `past` followed on its line by a type start in pastTable, not a pastName.
func (p *parser) pastKeyword() bool {
	next := p.pos + 1
	if !p.atWord(wordPast) || pastTable[p.peek(1)] == nil || !p.sameLine(p.pos, next) {
		return false
	}
	return !slices.ContainsFunc(pastNames, func(w string) bool { return p.wordAt(next, w) })
}

func (p *parser) namedType() Type {
	start := p.pos
	name := p.qualifiedIdent()
	if name == nil {
		return &BadType{Bounds: p.badBounds(start)}
	}
	return &NamedType{Name: name, Bounds: p.from(start)}
}

// listType is "[" type "]".
func (p *parser) listType() Type {
	start := p.next()
	elem := p.inType(p.typ)
	if p.expect(TokRBrack) == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	return &ListType{Elem: elem, Bounds: p.from(start)}
}

// mapType is "{" type ":" type "}", or depMapType "{" IDENT "in" expr ":" type "}".
func (p *parser) mapType() Type {
	start := p.next()
	header := p.header
	p.header = false
	defer func() { p.header = header }()
	if p.at(TokIdent) && p.peek(1) == KwIn {
		d := &DepMapType{Var: p.ref()}
		p.next()
		d.Domain = p.expr()
		if p.expect(TokColon) == NoTok {
			return &BadType{Bounds: p.badBounds(start)}
		}
		d.Value = p.typ()
		if p.expect(TokRBrace) == NoTok {
			return &BadType{Bounds: p.badBounds(start)}
		}
		d.Bounds = p.from(start)
		return d
	}
	m := &MapType{Key: p.typ()}
	if p.expect(TokColon) == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	m.Value = p.typ()
	if p.expect(TokRBrace) == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	m.Bounds = p.from(start)
	return m
}

// tableType is [ "stable" ] "table" qualifiedIdent.
func (p *parser) tableType() Type {
	start := p.pos
	t := &TableType{Stable: p.accept(KwStable)}
	if p.expect(KwTable) == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	if t.Name = p.qualifiedIdent(); t.Name == nil {
		return &BadType{Bounds: p.badBounds(start)}
	}
	t.Bounds = p.from(start)
	return t
}

// refType is "ref" qualifiedIdent (GRM-04).
func (p *parser) refType() Type {
	start := p.next()
	name := p.qualifiedIdent()
	if name == nil {
		return &BadType{Bounds: p.badBounds(start)}
	}
	return &RefType{Name: name, Bounds: p.from(start)}
}

func (p *parser) anyType() Type {
	t := p.next()
	return &AnyType{Bounds: Bounds{From: t, To: t}}
}

func (p *parser) literalType() Type {
	start := p.pos
	s := p.strLit()
	if s == nil {
		return &BadType{Bounds: p.badBounds(start)}
	}
	return &LiteralType{Value: s, Bounds: p.from(start)}
}

// parenType is "(" type ")"; the formatter keeps its parentheses.
func (p *parser) parenType() Type {
	start := p.next()
	inner := p.inType(p.typ)
	if p.expect(TokRParen) == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	return &ParenType{Type: inner, Bounds: p.from(start)}
}

// inType parses a type with header mode off, as inside any bracket (GRAMMAR.md §6.1).
func (p *parser) inType(t func() Type) Type {
	header := p.header
	p.header = false
	defer func() { p.header = header }()
	return t()
}

// fnType is "fn" "(" [ type { "," type } [ "," ] ] ")" "->" primType [ "?" ] (GRM-15).
func (p *parser) fnType() Type {
	start := p.next()
	f := &FnType{}
	f.Parens = p.parenList(TokLParen, TokRParen, func() { f.Params = append(f.Params, p.typ()) })
	if f.Parens.Close == NoTok || p.expect(TokArrow) == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	res := p.pos
	f.Result = p.primType()
	if p.at(TokQuestion) {
		p.next()
		f.Result = &OptionalType{Elem: f.Result, Bounds: p.from(res)}
	}
	f.Bounds = p.from(start)
	return f
}

// assetType is "asset" "(" stringLit [ "," "ext" ":" "[" extName { "," extName } [ "," ] "]" ]
// [ "," ] ")", an extName being IDENT or stringLit.
func (p *parser) assetType() Type {
	start := p.next()
	a := &AssetType{}
	if a.Parens.Open = p.expect(TokLParen); a.Parens.Open == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	if a.Dir = p.strLit(); a.Dir == nil {
		return &BadType{Bounds: p.badBounds(start)}
	}
	if p.at(TokComma) && p.wordAt(p.pos+1, wordExt) {
		p.next()
		p.next()
		if p.expect(TokColon) == NoTok {
			return &BadType{Bounds: p.badBounds(start)}
		}
		a.Brackets = p.parenList(TokLBrack, TokRBrack, func() {
			if n := p.extName(); n != nil {
				a.Exts = append(a.Exts, n)
			}
		})
		if a.Brackets.Close == NoTok {
			return &BadType{Bounds: p.badBounds(start)}
		}
	}
	p.accept(TokComma)
	if a.Parens.Close = p.expect(TokRParen); a.Parens.Close == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	a.Bounds = p.from(start)
	return a
}

func (p *parser) extName() NameLit {
	if startsString[p.kind()] {
		if s := p.strLit(); s != nil {
			return s
		}
		return nil
	}
	if id := p.ref(); id != nil {
		return id
	}
	return nil
}

// matchType is "match" headerExpr BraceList( typeArm ), typeArm being patterns "=>" type.
func (p *parser) matchType() Type {
	start := p.next()
	m := &MatchType{Scrutinee: p.headerExpr()}
	m.Braces = p.braceList(func() bool {
		at := p.pos
		arm := &TypeArm{Patterns: p.patterns()}
		if arm.Patterns == nil || p.expect(TokFatArrow) == NoTok {
			return false
		}
		arm.Type = p.typ()
		arm.Bounds = p.from(at)
		m.Arms = append(m.Arms, arm)
		return true
	}, nil)
	if m.Braces.Close == NoTok {
		return &BadType{Bounds: p.badBounds(start)}
	}
	m.Bounds = p.from(start)
	return m
}

// patterns is pattern { "," pattern }; nil when one did not parse.
func (p *parser) patterns() []*Pattern {
	var out []*Pattern
	for {
		pt := p.pattern()
		if pt == nil {
			return nil
		}
		out = append(out, pt)
		if p.accept(TokComma) == NoTok {
			return out
		}
	}
}

// pattern is "_" | "none" | qualifiedWord [ "(" binder ")" ] (GRAMMAR.md §5.11).
func (p *parser) pattern() *Pattern {
	start := p.pos
	switch p.kind() {
	case TokUnderscore, KwNone:
		k := p.kind()
		t := p.next()
		return &Pattern{Bounds: Bounds{From: t, To: t}, Keyword: k}
	default:
	}
	pt := &Pattern{Name: p.qualifiedWord()}
	if pt.Name == nil {
		return nil
	}
	if p.at(TokLParen) {
		pt.Parens.Open = p.next()
		if pt.Binder = p.binder(); pt.Binder == nil {
			return nil
		}
		if pt.Parens.Close = p.expect(TokRParen); pt.Parens.Close == NoTok {
			return nil
		}
	}
	pt.Bounds = p.from(start)
	return pt
}
