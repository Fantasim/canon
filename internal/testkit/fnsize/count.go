package fnsize

import "slices"

// counter counts the statements at the top level of a C++ or TypeScript body: a ";" outside any
// bracket ends one, and so does the "}" of a control block. An else, catch or finally joins the
// statement before it. The count is syntactic, not a parse.
type counter struct {
	toks    []token
	match   []int
	n       int
	started bool   // tokens of an unfinished statement have been seen
	lastEnd bool   // the previous event closed a statement
	first   string // the first token of the unfinished statement
}

// countStatements counts the statements of toks[lo:hi].
func countStatements(toks []token, match []int, lo, hi int) int {
	c := &counter{toks: toks, match: match}
	for i := lo; i < hi; i++ {
		i = c.step(i, lo)
	}
	if c.started {
		c.n++
	}
	return c.n
}

// step consumes the token at i, and what it opens, and returns the index of the last token used.
func (c *counter) step(i, lo int) int {
	t := c.toks[i]
	switch {
	case isPunct(t, punctSemicolon):
		c.semicolon()
	case isPunct(t, punctOpenParen), isPunct(t, punctOpenBracket):
		c.touch(t.text)
		return c.match[i]
	case isPunct(t, punctOpenBrace):
		c.block(i, lo)
		return c.match[i]
	case t.kind == kindIdent && slices.Contains(continuationWords, t.text):
		if c.lastEnd {
			c.n--
		}
		c.touch(t.text)
	default:
		c.touch(t.text)
	}
	return i
}

// touch records a token of the statement under way.
func (c *counter) touch(text string) {
	if !c.started {
		c.started, c.first = true, text
	}
	c.lastEnd = false
}

func (c *counter) semicolon() {
	if c.lastEnd {
		return
	}
	c.n++
	c.started, c.lastEnd = false, true
}

// block handles the brace at i: a statement block ends its statement, a do block waits for its
// while, any other brace (a list, a lambda body, an object) is part of the statement under way.
func (c *counter) block(i, lo int) {
	prev := ""
	if i > lo {
		prev = c.toks[i-1].text
	}
	switch {
	case prev == wordDo:
		c.touch(punctOpenBrace)
	case c.endsStatement(prev):
		c.n++
		c.started, c.lastEnd = false, true
	default:
		c.touch(punctOpenBrace)
	}
}

// endsStatement reports whether a block after prev closes the statement it belongs to.
func (c *counter) endsStatement(prev string) bool {
	if !c.started {
		return true
	}
	if slices.Contains(blockLeaders, prev) {
		return true
	}
	return prev == punctCloseParen && slices.Contains(controlWords, c.first)
}
