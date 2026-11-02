package fnsize

import "slices"

// scanTS finds the outermost function bodies of a TypeScript file: every "{" outside a function
// is tested, and a function's whole body is skipped, so what it holds belongs to it.
func scanTS(toks []token, match []int) []Func {
	var funcs []Func
	for i := 1; i < len(toks); i++ {
		if !isPunct(toks[i], punctOpenBrace) {
			continue
		}
		if name, ok := bodyName(toks, match, i); ok {
			funcs = append(funcs, newFunc(toks, match, name, i))
			i = match[i]
		}
	}
	return funcs
}

// bodyName reports whether the "{" at i opens a function body, and the function's name: it
// follows an arrow, or a parameter list (with an optional return type) that a name precedes.
func bodyName(toks []token, match []int, i int) (string, bool) {
	if prev := toks[i-1]; prev.kind == kindPunct && !slices.Contains(bodyEnders, prev.text) && prev.text != punctFatArrow {
		return "", false
	}
	if isPunct(toks[i-1], punctFatArrow) {
		if typeLiteralAt(toks, i) {
			return "", false
		}
		return arrowName(toks, match, i-1), true
	}
	closeParen, ok := paramsClose(toks, match, i-1)
	if !ok {
		return "", false
	}
	return methodName(toks, match[closeParen])
}

// typeLiteralAt reports whether the brace after an arrow opens an object type ({ a: T }) rather
// than a body: its first member is a name followed by a colon or an optional mark.
func typeLiteralAt(toks []token, i int) bool {
	if i+memberColonOffset >= len(toks) || (toks[i+1].kind != kindIdent && toks[i+1].kind != kindString) {
		return false
	}
	next := toks[i+memberColonOffset]
	if isPunct(next, punctColon) {
		return true
	}
	return isPunct(next, tsOptional) && i+memberOptColonOffset < len(toks) && isPunct(toks[i+memberOptColonOffset], punctColon)
}

// isTypeToken reports whether t can sit in a type annotation between a ")" and a body: a word, a
// literal or type punctuation.
func isTypeToken(t token) bool {
	return t.kind != kindPunct || slices.Contains(tsTypePunct, t.text)
}

// skipType walks back from p over a type annotation and returns the index of the token before
// it. An object type or a function type is stepped over whole when an operator of a type
// precedes its opener.
func skipType(toks []token, match []int, p int) int {
	j := p
	for j >= 0 {
		t := toks[j]
		switch {
		case t.kind == kindPunct && slices.Contains(typeGroupEnds, t.text) && groupInType(toks, match[j]):
			j = match[j]
		case !isTypeToken(t):
			return j
		}
		j--
	}
	return j
}

// groupInType reports whether the group opening at o is part of a type.
func groupInType(toks []token, o int) bool {
	return o > 0 && toks[o-1].kind == kindPunct && slices.Contains(typeLeaders, toks[o-1].text)
}

// paramsClose finds the ")" that ends the parameter list when p is that ")" or the end of a
// return type annotation.
func paramsClose(toks []token, match []int, p int) (int, bool) {
	if p >= 0 && isPunct(toks[p], punctCloseParen) {
		return p, true
	}
	j := skipType(toks, match, p)
	if j >= 1 && isPunct(toks[j], punctColon) && isPunct(toks[j-1], punctCloseParen) {
		return j - 1, true
	}
	return 0, false
}

// methodName names the function whose parameter list opens at o: the identifier before it
// (and before its type parameters). A control statement is not a function.
func methodName(toks []token, o int) (string, bool) {
	q := o - 1
	if q >= 0 && isPunct(toks[q], punctGreater) {
		q = openAngle(toks, q) - 1
	}
	if q < 0 || toks[q].kind != kindIdent {
		return anonymousName, true
	}
	switch {
	case slices.Contains(tsControlWords, toks[q].text):
		return "", false
	case toks[q].text == wordFunction:
		return anonymousName, true
	}
	return toks[q].text, true
}

// arrowName names an arrow function from the binding or the property before its parameters.
func arrowName(toks []token, match []int, arrow int) string {
	o := arrow - 1
	if o < 0 {
		return anonymousName
	}
	if closeParen, ok := paramsClose(toks, match, o); ok {
		o = match[closeParen]
	} else if toks[o].kind != kindIdent {
		return anonymousName
	}
	r := o - 1
	if r >= 0 && isWord(toks[r], wordAsync) {
		r--
	}
	if r >= 1 && (isPunct(toks[r], punctAssign) || isPunct(toks[r], punctColon)) && toks[r-1].kind == kindIdent {
		return toks[r-1].text
	}
	return anonymousName
}
