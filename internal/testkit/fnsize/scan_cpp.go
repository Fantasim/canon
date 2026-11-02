package fnsize

import "slices"

// angleDelta moves the template-argument depth: a "<" or ">" that is not an operator's.
func angleDelta(t token, afterOperator bool) int {
	switch {
	case afterOperator:
		return 0
	case isPunct(t, punctLess):
		return 1
	case isPunct(t, punctGreater):
		return -1
	}
	return 0
}

// functionOrOther decides a header whose first parenthesis, outside template arguments, is at p:
// it is a function when the brace follows a parameter list, a qualifier, an initializer list or
// a trailing return type.
func functionOrOther(header []token, p int) (cppKind, string, error) {
	if slices.ContainsFunc(header[p:], func(t token) bool { return isWord(t, wordTry) }) {
		return cppOther, "", refusal(reasonTry)
	}
	last := header[len(header)-1]
	ok := isPunct(last, punctCloseParen) || isPunct(last, punctCloseBrace) ||
		last.kind != kindString && slices.Contains(cppQualifiers, last.text) ||
		slices.ContainsFunc(header[p:], func(t token) bool { return isPunct(t, punctThinArrow) })
	if !ok {
		return cppOther, "", nil
	}
	name := cppName(header, p)
	if slices.Contains(controlNames, name) {
		return cppOther, "", refusal(reasonControl)
	}
	return cppFunction, name, nil
}

// afterTemplate returns the index past a leading template<...> parameter list.
func afterTemplate(header []token) int {
	if len(header) < templatePrefixLen || !isWord(header[0], wordTemplate) || !isPunct(header[1], punctLess) {
		return 0
	}
	depth := 0
	for j := 1; j < len(header); j++ {
		depth += angleDelta(header[j], false)
		if depth == 0 {
			return j + 1
		}
	}
	return len(header)
}

// cppName is the (qualified) name before the parameter list that opens at p.
func cppName(header []token, p int) string {
	i := p - 1
	name := anonymousName
	switch {
	case i < 0:
		return name
	case isWord(header[i], wordOperator):
		name = wordOperator
		if p+1 < len(header) && isPunct(header[p+1], punctCloseParen) {
			name += punctOpenParen + punctCloseParen
		}
		i--
	case header[i].kind == kindPunct && i > 0 && isWord(header[i-1], wordOperator):
		name = wordOperator + header[i].text
		i -= operatorTokens
	case header[i].kind == kindIdent:
		name = header[i].text
		i--
	default:
		return name
	}
	if i >= 0 && isPunct(header[i], punctTilde) {
		name, i = punctTilde+name, i-1
	}
	return qualify(header, i, name)
}

// qualify prepends the Class:: parts that precede index i.
func qualify(header []token, i int, name string) string {
	for i >= 1 && isPunct(header[i], punctScope) {
		i--
		if isPunct(header[i], punctGreater) {
			i = openAngle(header, i) - 1
		}
		if i < 0 || header[i].kind != kindIdent {
			break
		}
		name = header[i].text + punctScope + name
		i--
	}
	return name
}

// openAngle returns the index of the "<" that pairs the ">" at i, looking back; i when none.
func openAngle(toks []token, i int) int {
	depth := 0
	for j := i; j >= 0; j-- {
		depth -= angleDelta(toks[j], false)
		if depth == 0 {
			return j
		}
	}
	return i
}
