package catalog

import (
	"fmt"
	"strings"
)

// table is one Markdown table: its header line and its rows, separator excluded.
type table struct {
	header string
	sep    string
	line   int
	rows   []row
}

type row struct {
	cells []string
	line  int
}

// section is the text under one "## " heading; the zero section holds what precedes the
// first heading.
type section struct {
	title  string
	line   int
	tables []table
	lines  []numbered
}

type numbered struct {
	text string
	line int
}

// readSections splits a Markdown document into its level-2 sections and their tables.
// Lines inside fenced code blocks are neither headings nor table rows.
func readSections(md []byte) []section {
	secs := []section{{}}
	fenced := false
	for i, text := range strings.Split(string(md), lineBreak) {
		n := numbered{text: text, line: i + 1}
		if strings.HasPrefix(text, codeFence) {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(text, sectionPrefix) {
			secs = append(secs, section{title: strings.TrimPrefix(text, sectionPrefix), line: n.line})
			continue
		}
		cur := &secs[len(secs)-1]
		if fenced {
			n.text = ""
		}
		cur.lines = append(cur.lines, n)
	}
	for i := range secs {
		secs[i].tables = readTables(secs[i].lines)
	}
	return secs
}

// readTables groups consecutive "|" lines into tables; the second line of a table is its
// separator row and is dropped.
func readTables(lines []numbered) []table {
	var out []table
	inTable := false
	for i, l := range lines {
		if !strings.HasPrefix(l.text, cellSep) {
			inTable = false
			continue
		}
		if !inTable {
			out = append(out, table{header: l.text, line: l.line})
			inTable = true
			continue
		}
		t := &out[len(out)-1]
		if i > 0 && lines[i-1].line == t.line {
			t.sep = l.text
			continue
		}
		t.rows = append(t.rows, row{cells: splitCells(l.text), line: l.line})
	}
	return out
}

// splitCells returns the trimmed cells of a table line; an escaped "\|" stays in its cell.
func splitCells(line string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), cellSep), cellSep)
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(inner); i++ {
		switch {
		case inner[i] == escapeChar && i+1 < len(inner) && inner[i+1] == cellSep[0]:
			cur.WriteByte(cellSep[0])
			i++
		case inner[i] == cellSep[0]:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(inner[i])
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

// findTable returns the one table of the document whose header is exactly header.
func findTable(secs []section, header string) (table, error) {
	var found []table
	for _, s := range secs {
		for _, t := range s.tables {
			if t.header == header {
				found = append(found, t)
			}
		}
	}
	if len(found) != 1 {
		return table{}, fmt.Errorf("%w: %d tables with the header %s, want exactly one", errTable, len(found), header)
	}
	return found[0], nil
}

// checkWidth fails when the separator row is malformed or a row does not have want cells.
func checkWidth(t table, want int) error {
	if strings.Trim(t.sep, sepChars) != "" || strings.Count(t.sep, cellSep) != want+1 {
		return fmt.Errorf("%w: line %d: malformed separator row %q", errTable, t.line+1, t.sep)
	}
	for _, r := range t.rows {
		if len(r.cells) != want {
			return fmt.Errorf("%w: line %d: %d cells, want %d", errTable, r.line, len(r.cells), want)
		}
	}
	return nil
}

// codeSpan returns the text of a cell written as one code span.
func codeSpan(cell string, line int) (string, error) {
	inner, ok := strings.CutPrefix(cell, backtick)
	inner, ok2 := strings.CutSuffix(inner, backtick)
	if !ok || !ok2 || len(cell) < len(backtick+backtick) || strings.Contains(inner, backtick) {
		return "", fmt.Errorf("%w: line %d: %q is not one code span", errTable, line, cell)
	}
	return inner, nil
}

// headerWidth is the number of cells of a table's header row.
func headerWidth(header string) int {
	return strings.Count(header, cellSep) - 1
}
