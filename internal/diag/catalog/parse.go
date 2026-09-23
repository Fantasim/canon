package catalog

import (
	"errors"
	"fmt"
	"strings"
)

// Parse reads spec/ERRORS.md and validates it against the plan's package table (ERRORS.md §2.1).
func Parse(errorsMD, planMD []byte) (*Catalog, error) {
	pkgs, err := Packages(planMD)
	if err != nil {
		return nil, err
	}
	secs := readSections(errorsMD)
	c := &Catalog{}
	var errs []error
	c.ArgTypes, err = parseArgTypes(secs)
	errs = append(errs, err)
	c.Kinds, err = parseKinds(secs)
	errs = append(errs, err)
	c.Runtime, err = parseRuntime(secs)
	errs = append(errs, err)
	c.Codes, err = parseCodeSections(secs)
	errs = append(errs, err)
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if err := validate(c, packageNames(pkgs), secs); err != nil {
		return nil, err
	}
	return c, nil
}

func parseArgTypes(secs []section) ([]ArgType, error) {
	t, err := tableOf(secs, typesHeader)
	if err != nil {
		return nil, err
	}
	out := make([]ArgType, 0, len(t.rows))
	for _, r := range t.rows {
		name, err1 := codeSpan(r.cells[colTypeName], r.line)
		goType, err2 := codeSpan(r.cells[colTypeGo], r.line)
		if err := errors.Join(err1, err2); err != nil {
			return nil, err
		}
		out = append(out, ArgType{Name: name, GoType: goType})
	}
	return out, nil
}

func parseKinds(secs []section) ([]Kind, error) {
	t, err := tableOf(secs, kindsHeader)
	if err != nil {
		return nil, err
	}
	out := make([]Kind, 0, len(t.rows))
	for _, r := range t.rows {
		name, err := codeSpan(r.cells[colKindName], r.line)
		if err != nil {
			return nil, err
		}
		k := Kind{Name: name, Word: r.cells[colKindWord], line: r.line}
		for _, used := range splitList(r.cells[colKindUsedBy]) {
			code, err := codeSpan(used, r.line)
			if err != nil {
				return nil, err
			}
			k.UsedBy = append(k.UsedBy, code)
		}
		out = append(out, k)
	}
	return out, nil
}

func parseRuntime(secs []section) ([]RuntimeText, error) {
	t, err := tableOf(secs, runtimeHeader)
	if err != nil {
		return nil, err
	}
	out := make([]RuntimeText, 0, len(t.rows))
	for _, r := range t.rows {
		text, err := codeSpan(r.cells[colRuntimeText], r.line)
		if err != nil {
			return nil, err
		}
		out = append(out, RuntimeText{Code: r.cells[colCode], Text: text})
	}
	return out, nil
}

// tableOf finds the one table with header and checks its rows against it.
func tableOf(secs []section, header string) (table, error) {
	t, err := findTable(secs, header)
	if err != nil {
		return table{}, err
	}
	return t, checkWidth(t, headerWidth(header))
}

// parseCodeSections reads the codes and messages tables of every section whose title
// starts with a code range; such a section holds exactly those two tables, in that order.
func parseCodeSections(secs []section) ([]Code, error) {
	var codes []Code
	for _, s := range secs {
		if !reRangeTitle.MatchString(s.title) {
			continue
		}
		got, err := parseCodeSection(s)
		if err != nil {
			return nil, err
		}
		codes = append(codes, got...)
	}
	return codes, nil
}

func parseCodeSection(s section) ([]Code, error) {
	if len(s.tables) != len(codeSectionHeaders) {
		return nil, fmt.Errorf("%w: line %d: section %q has %d tables, want the codes and messages tables", errTable, s.line, s.title, len(s.tables))
	}
	for i, h := range codeSectionHeaders {
		if s.tables[i].header != h {
			return nil, fmt.Errorf("%w: line %d: header %s, want %s", errTable, s.tables[i].line, s.tables[i].header, h)
		}
		if err := checkWidth(s.tables[i], headerWidth(h)); err != nil {
			return nil, err
		}
	}
	codes, err := codeRows(s)
	if err != nil {
		return nil, err
	}
	return codes, attachMessages(codes, s.tables[1].rows)
}

// codeRows reads the codes table of a section; each code must fall in the title's range.
func codeRows(s section) ([]Code, error) {
	ranges := titleRanges(s.title)
	var codes []Code
	for _, r := range s.tables[0].rows {
		c := Code{
			ID: r.cells[colCode], Severity: r.cells[colSeverity], Package: r.cells[colPackage],
			Owner: r.cells[colOwner], Meaning: r.cells[colMeaning], line: r.line,
		}
		if !inRanges(c.ID, ranges) {
			return nil, fmt.Errorf("%w: line %d: %s is outside the range of section %q", errCode, r.line, c.ID, s.title)
		}
		codes = append(codes, c)
	}
	return codes, nil
}

// attachMessages gives each code its message rows, which follow the codes table's order.
func attachMessages(codes []Code, rows []row) error {
	at := 0
	for _, r := range rows {
		for at < len(codes) && codes[at].ID != r.cells[colCode] {
			at++
		}
		if at == len(codes) {
			return fmt.Errorf("%w: line %d: message row of %s names no code of its section, or breaks the codes table's order", errMessage, r.line, r.cells[colCode])
		}
		m, err := messageRow(r)
		if err != nil {
			return err
		}
		codes[at].Messages = append(codes[at].Messages, m)
	}
	return nil
}

func messageRow(r row) (Message, error) {
	tpl, err := codeSpan(r.cells[colTemplate], r.line)
	if err != nil {
		return Message{}, err
	}
	m := Message{Variant: r.cells[colVariant], Template: tpl, line: r.line}
	if m.Variant == noneCell {
		m.Variant = ""
	}
	if r.cells[colArgs] == noneCell {
		return m, nil
	}
	for _, a := range splitList(r.cells[colArgs]) {
		name, typ, ok := strings.Cut(a, argSep)
		if !ok || strings.Contains(typ, argSep) {
			return Message{}, fmt.Errorf("%w: line %d: argument %q is not name:Type", errArgs, r.line, a)
		}
		m.Args = append(m.Args, Arg{Name: name, Type: typ})
	}
	return m, nil
}

// splitList splits a ", "-separated cell.
func splitList(cell string) []string {
	if cell == "" {
		return nil
	}
	return strings.Split(cell, listSep)
}
