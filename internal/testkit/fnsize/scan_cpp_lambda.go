package fnsize

import "slices"

// lambdas measures the outermost lambda bodies of the declaration toks[h:end], named after the
// variable the declaration initializes. A lambda inside a lambda belongs to the outer one.
func (s *cppScan) lambdas(h, end int) {
	name := initializedName(s.toks[h:end])
	for i := h; i < end; i++ {
		if isPunct(s.toks[i], punctOpenBrace) && s.lambdaBodyAt(h, i) {
			s.funcs = append(s.funcs, newFunc(s.toks, s.match, name, i))
			i = s.match[i]
		}
	}
}

// initializedName is the identifier before the first "=" outside template arguments.
func initializedName(decl []token) string {
	angle := 0
	for j, t := range decl {
		prevOp := j > 0 && isWord(decl[j-1], wordOperator)
		angle += angleDelta(t, prevOp)
		if angle == 0 && isPunct(t, punctAssign) && !prevOp && j > 0 && decl[j-1].kind == kindIdent {
			return decl[j-1].text
		}
	}
	return anonymousName
}

// lambdaBodyAt reports whether the "{" at i, in a declaration that began at h, follows a capture
// list "[...]" (then an optional parameter list, specifiers, return type) that is not a subscript.
func (s *cppScan) lambdaBodyAt(h, i int) bool {
	j := s.skipSpecs(h, s.skipReturnType(h, s.skipSpecs(h, i-1)))
	if j >= h && isPunct(s.toks[j], punctCloseParen) {
		j = s.skipSpecs(h, s.match[j]-1)
	}
	if j < h || !isPunct(s.toks[j], punctCloseBracket) {
		return false
	}
	open := s.match[j]
	return open > h && s.toks[open-1].kind == kindPunct && !slices.Contains(subscriptEnds, s.toks[open-1].text)
}

// skipSpecs steps back over mutable, constexpr, consteval and noexcept.
func (s *cppScan) skipSpecs(h, j int) int {
	for j >= h && s.toks[j].kind == kindIdent && slices.Contains(lambdaSpecs, s.toks[j].text) {
		j--
	}
	return j
}

// skipReturnType steps back over "-> type" to before the arrow, when there is one.
func (s *cppScan) skipReturnType(h, j int) int {
	for k := j; k >= h; k-- {
		t := s.toks[k]
		if isPunct(t, punctThinArrow) {
			return k - 1
		}
		if t.kind == kindPunct && !slices.Contains(returnTypePunct, t.text) {
			return j
		}
	}
	return j
}
