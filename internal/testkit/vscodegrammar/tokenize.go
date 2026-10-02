package vscodegrammar

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type token struct {
	line, start, end int
	scopes           []string
}

type candidate struct {
	r      *rule
	loc    []int
	isEnd  bool
	isBeg  bool
	groups map[int]string
}

type tokenizer struct {
	g     *Grammar
	stack []*rule
	out   []token
}

func newTokenizer(g *Grammar) *tokenizer { return &tokenizer{g: g} }

// scopes is the scope list in force; the top frame's contentName counts only between its
// begin and its end.
func (tk *tokenizer) scopes(withTopContent bool) []string {
	s := []string{tk.g.scope}
	for i, r := range tk.stack {
		if r.name != "" {
			s = append(s, r.name)
		}
		if r.content != "" && (withTopContent || i < len(tk.stack)-1) {
			s = append(s, r.content)
		}
	}
	return s
}

func (tk *tokenizer) emit(line, start, end int, scopes []string, extra string) {
	if end <= start {
		return
	}
	if extra != "" {
		scopes = append(append([]string{}, scopes...), extra)
	}
	if n := len(tk.out); n > 0 {
		last := &tk.out[n-1]
		if last.line == line && last.end == start && strings.Join(last.scopes, " ") == strings.Join(scopes, " ") {
			last.end = end
			return
		}
	}
	tk.out = append(tk.out, token{line, start, end, append([]string{}, scopes...)})
}

// search runs re on line from pos; a ^ pattern is tried at column 0 only (doc.go).
func search(re *regexp.Regexp, line string, pos int) []int {
	if pos > 0 && strings.HasPrefix(re.String(), lineAnchor) {
		return nil
	}
	loc := re.FindStringSubmatchIndex(line[pos:])
	for i := range loc {
		if loc[i] >= 0 {
			loc[i] += pos
		}
	}
	return loc
}

// next is the leftmost candidate; the end pattern wins ties, then pattern order.
func (tk *tokenizer) next(line string, pos int) (candidate, bool) {
	var best candidate
	found := false
	consider := func(c candidate) {
		if c.loc != nil && (!found || c.loc[0] < best.loc[0]) {
			best, found = c, true
		}
	}
	patterns := tk.g.root.flatten()
	if n := len(tk.stack); n > 0 {
		top := tk.stack[n-1]
		patterns = top.flatten()
		consider(candidate{r: top, loc: search(top.end, line, pos), isEnd: true, groups: top.endCaps})
	}
	for _, r := range patterns {
		if r.match != nil {
			c := candidate{r: r, loc: search(r.match, line, pos), groups: r.caps}
			consider(nonEmpty(c))
			continue
		}
		c := candidate{r: r, loc: search(r.begin, line, pos), isBeg: true, groups: r.begCaps}
		consider(nonEmpty(c))
	}
	return best, found
}

func nonEmpty(c candidate) candidate {
	if c.loc != nil && c.loc[0] == c.loc[1] {
		c.loc = nil
	}
	return c
}

// emitMatch writes a match as one token run, its captures carrying their own scope.
func (tk *tokenizer) emitMatch(n int, c candidate, base []string) error {
	cursor := c.loc[0]
	for _, g := range slices.Sorted(maps.Keys(c.groups)) {
		if g*pairWidth+1 >= len(c.loc) || c.loc[g*pairWidth] < 0 {
			continue
		}
		gs, ge := c.loc[g*pairWidth], c.loc[g*pairWidth+1]
		if gs < cursor {
			return fmt.Errorf("%w: group %d", errNestedCapture, g)
		}
		tk.emit(n, cursor, gs, base, "")
		tk.emit(n, gs, ge, base, c.groups[g])
		cursor = ge
	}
	tk.emit(n, cursor, c.loc[1], base, "")
	return nil
}

func (tk *tokenizer) apply(n int, c candidate) error {
	switch {
	case c.isEnd:
		base := tk.scopes(false)
		err := tk.emitMatch(n, c, base)
		tk.stack = tk.stack[:len(tk.stack)-1]
		return err
	case c.isBeg:
		tk.stack = append(tk.stack, c.r)
		return tk.emitMatch(n, c, tk.scopes(false))
	}
	base := tk.scopes(true)
	if c.r.name != "" {
		base = append(base, c.r.name)
	}
	return tk.emitMatch(n, c, base)
}

// line tokenises one line (without its newline); the stack carries over to the next line.
func (tk *tokenizer) line(n int, text string) error {
	pos := 0
	for pos <= len(text) {
		c, ok := tk.next(text, pos)
		if !ok {
			tk.emit(n, pos, len(text), tk.scopes(true), "")
			return nil
		}
		tk.emit(n, pos, c.loc[0], tk.scopes(true), "")
		if err := tk.apply(n, c); err != nil {
			return err
		}
		pos = c.loc[1]
	}
	return nil
}

// Tokenize runs a whole text and renders the snapshot: one line per scoped token, as
// line:startByte-endByte "text" scopes.
func (g *Grammar) Tokenize(text string) (string, error) {
	tk := newTokenizer(g)
	lines := strings.Split(strings.ReplaceAll(text, carriageStr+lineSeparator, lineSeparator), lineSeparator)
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, l := range lines {
		if err := tk.line(i+1, l); err != nil {
			return "", fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return render(lines, tk.out), nil
}

func render(lines []string, toks []token) string {
	var b strings.Builder
	for _, t := range toks {
		scopes := t.scopes[1:]
		if len(scopes) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%d:%d-%d %s %s\n", t.line, t.start, t.end,
			strconv.Quote(lines[t.line-1][t.start:t.end]), strings.Join(scopes, " "))
	}
	return b.String()
}
