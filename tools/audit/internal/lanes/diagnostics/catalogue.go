package diagnostics

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// entry is one code of the catalogue: its owning package and the line of its codes row.
type entry struct {
	code, pkg string
	line      int
}

// template is one message of the catalogue, or one text signalled by generated code.
type template struct {
	code, text string
}

type catalogue struct {
	codes     []entry
	templates []template
}

// readCatalogue parses path; a repository without one has nothing to check (nil, nil).
func readCatalogue(path string) (*catalogue, error) {
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCatalogue, err)
	}
	c := parseCatalogue(string(src))
	if len(c.codes) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoCodes, path)
	}
	return c, nil
}

// parseCatalogue reads the rows of every codes, messages and runtime-text table.
func parseCatalogue(src string) *catalogue {
	c := &catalogue{}
	table := ""
	for i, line := range strings.Split(src, lineBreak) {
		switch {
		case line == headerCodes || line == headerMessages || line == headerRuntime:
			table = line
		case !strings.HasPrefix(line, tableRow):
			table = ""
		case table != "" && !strings.HasPrefix(line, tableRule):
			c.addRow(table, cells(line), i+1)
		}
	}
	return c
}

func (c *catalogue) addRow(table string, row []string, line int) {
	if len(row) <= colText || !reCode.MatchString(row[colCode]) {
		return
	}
	switch {
	case table == headerCodes && len(row) > colPackage:
		c.codes = append(c.codes, entry{code: row[colCode], pkg: row[colPackage], line: line})
	case table == headerMessages && len(row) > colTemplate:
		c.templates = append(c.templates, template{code: row[colCode], text: row[colTemplate]})
	case table == headerRuntime:
		c.templates = append(c.templates, template{code: row[colCode], text: row[colText]})
	}
}

// cells splits a table row, each cell trimmed of spaces and of the code span around it.
func cells(line string) []string {
	parts := strings.Split(strings.Trim(line, tableRow), tableRow)
	for i, p := range parts {
		parts[i] = strings.Trim(strings.TrimSpace(p), codeSpan)
	}
	return parts
}

// segment is a fixed run of a template's text, long enough that a literal holding it
// holds the message.
type segment struct {
	code, text string
}

// segments are the fixed runs of every template with at least minWords words, trimmed of
// the spaces, quotes and punctuation a caller's own format would supply.
func (c *catalogue) segments(minWords int) []segment {
	var out []segment
	seen := map[string]bool{}
	for _, t := range c.templates {
		for _, run := range fixedRuns(t.text) {
			run = strings.Trim(run, segmentTrim)
			if len(strings.Fields(run)) < minWords || seen[run] {
				continue
			}
			seen[run] = true
			out = append(out, segment{code: t.code, text: run})
		}
	}
	return out
}

// fixedRuns cuts a template at its placeholders and line breaks, reading escapes as text.
func fixedRuns(tmpl string) []string {
	var (
		out []string
		cur strings.Builder
	)
	for rest := tmpl; rest != ""; {
		if text, n := escape(rest); n > 0 {
			cur.WriteString(text)
			rest = rest[n:]
			continue
		}
		if n := breakLen(rest); n > 0 {
			out = append(out, cur.String())
			cur.Reset()
			rest = rest[n:]
			continue
		}
		cur.WriteByte(rest[0])
		rest = rest[1:]
	}
	return append(out, cur.String())
}

func escape(s string) (string, int) {
	for _, e := range escapes {
		if strings.HasPrefix(s, e.seq) {
			return e.text, len(e.seq)
		}
	}
	return "", 0
}

// breakLen is the length of a line break or a placeholder opening s, else 0.
func breakLen(s string) int {
	if strings.HasPrefix(s, escLineBreak) {
		return len(escLineBreak)
	}
	if s[0] != braceOpen {
		return 0
	}
	if end := strings.Index(s, braceClose); end > 0 {
		return end + len(braceClose)
	}
	return 0
}
