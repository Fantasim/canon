package fnsize

import (
	"fmt"
	"slices"
)

// cppKind is what a "{" at declaration level opens.
type cppKind uint8

// cppScan walks the declaration levels of a file: global, namespaces, extern blocks and type
// bodies. Everything inside a function body is the function's.
type cppScan struct {
	toks  []token
	match []int
	funcs []Func
}

func scanCpp(toks []token, match []int) ([]Func, error) {
	s := &cppScan{toks: toks, match: match}
	err := s.scope(0, len(toks))
	return s.funcs, err
}

// scope reads the declarations of toks[lo:hi]; h is where the declaration being read began. The
// lambdas of a declaration are measured when its ";" closes it.
func (s *cppScan) scope(lo, hi int) error {
	h := lo
	for i := lo; i < hi; i++ {
		t := s.toks[i]
		var err error
		switch {
		case isPunct(t, punctSemicolon):
			s.lambdas(h, i)
			h = i + 1
		case isPunct(t, punctOpenParen), isPunct(t, punctOpenBracket):
			i = s.match[i]
		case t.kind == kindIdent && slices.Contains(cppAccessWords, t.text) && i+1 < hi && isPunct(s.toks[i+1], punctColon):
			h, i = i+labelTokens, i+1
		case isPunct(t, punctOpenBrace):
			i, h, err = s.brace(h, i)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// brace reads the "{" at i of the declaration that began at h, and returns the index of its
// closing brace and where the next declaration begins.
func (s *cppScan) brace(h, i int) (int, int, error) {
	end := s.match[i]
	kind, name, err := classifyCpp(s.toks[h:i])
	if err != nil {
		return end, h, fmt.Errorf("line %d: %w", s.toks[i].line, err)
	}
	switch kind {
	case cppScope:
		err = s.scope(i+1, end)
	case cppFunction:
		s.funcs = append(s.funcs, newFunc(s.toks, s.match, name, i))
	case cppOther:
		return end, h, nil
	}
	return end, end + 1, err
}

// refusal is the error of a header the scanner cannot classify: reading on could skip a function.
func refusal(reason string) error { return fmt.Errorf("%s: %w", reason, errUnclassified) }

// classifyCpp decides what the declaration header before a "{" opens, and the function's name.
// A shape it cannot tell apart from a function is refused.
func classifyCpp(header []token) (cppKind, string, error) {
	header = header[afterTemplate(header):]
	if len(header) > 1 && isWord(header[0], wordExtern) && header[1].kind == kindString {
		if len(header) > externBlockTokens {
			return cppOther, "", refusal(reasonExtern)
		}
		return cppScope, "", nil
	}
	angle := 0
	for j, t := range header {
		prevOp := j > 0 && isWord(header[j-1], wordOperator)
		angle += angleDelta(t, prevOp)
		switch {
		case angle > 0:
		case isWord(t, wordAlignas):
			return cppOther, "", refusal(reasonAlignas)
		case isPunct(t, punctOpenParen):
			return functionOrOther(header, j)
		case t.kind == kindIdent && slices.Contains(cppScopeWords, t.text):
			return typeHead(header[j+1:])
		case isPunct(t, punctAssign) && !prevOp:
			return cppOther, "", nil
		}
	}
	return cppOther, "", nil
}

// typeHead accepts what follows class, struct, union, enum or namespace when no parenthesis
// opens outside template arguments; one that does is a function with an elaborated return type.
func typeHead(rest []token) (cppKind, string, error) {
	angle := 0
	for j, t := range rest {
		angle += angleDelta(t, j > 0 && isWord(rest[j-1], wordOperator))
		if angle == 0 && isPunct(t, punctOpenParen) {
			return cppOther, "", refusal(reasonElaborated)
		}
	}
	return cppScope, "", nil
}
