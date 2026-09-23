package catalog

import (
	"fmt"
	"strings"
)

// Packages reads the package table (IMPLEMENTATION-PLAN.md §3) in order: a row imports rows above.
func Packages(plan []byte) ([]Package, error) {
	t, err := tableOf(readSections(plan), planHeader)
	if err != nil {
		return nil, err
	}
	out := make([]Package, 0, len(t.rows))
	seen := map[string]bool{}
	for _, r := range t.rows {
		p, err := planRow(r)
		if err != nil {
			return nil, err
		}
		if seen[p.Name] {
			return nil, fmt.Errorf("%w: line %d: package %s listed twice", errPlan, r.line, p.Name)
		}
		seen[p.Name] = true
		out = append(out, p)
	}
	return out, nil
}

// planRow reads one row: the first code span of the first cell names the package, and a
// note after it ("(top level)", "(package `canon`)") places it outside internal/.
func planRow(r row) (Package, error) {
	spans := codeSpans(r.cells[0])
	if len(spans) == 0 {
		return Package{}, fmt.Errorf("%w: line %d: no package name in %q", errPlan, r.line, r.cells[0])
	}
	name := strings.TrimSuffix(spans[0], pathSep)
	dir := internalDir + name
	if strings.TrimSpace(strings.TrimPrefix(r.cells[0], backtick+spans[0]+backtick)) != "" {
		dir = name
	}
	p := Package{Name: name, Dir: dir}
	for _, s := range codeSpans(r.cells[1]) {
		if strings.HasPrefix(s, name+pathSep) {
			p.Subs = append(p.Subs, dir+strings.TrimPrefix(s, name))
		}
	}
	return p, nil
}

// codeSpans lists the code spans of a cell, in order.
func codeSpans(cell string) []string {
	parts := strings.Split(cell, backtick)
	var out []string
	for i := 1; i < len(parts); i += codeSpanStride {
		out = append(out, parts[i])
	}
	return out
}

// packageNames is every package name a codes row may give: the rows and their sub-packages.
func packageNames(pkgs []Package) map[string]bool {
	out := map[string]bool{}
	for _, p := range pkgs {
		out[p.Name] = true
		for _, s := range p.Subs {
			out[p.Name+strings.TrimPrefix(s, p.Dir)] = true
		}
	}
	return out
}
