package tsgen

import (
	_ "embed"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
)

var (
	//go:embed runtime/helpers.ts.txt
	helperText string
	//go:embed runtime/decode.ts.txt
	decodeText string
	//go:embed text/marker.txt
	markerFormat string
	//go:embed text/conformance_doc.txt
	conformanceDocFormat string
)

// unit is one top-level declaration of a helper text, with the doc comment above it; group numbers the blocks the text separates by blank lines.
type unit struct {
	name    string
	text    string
	group   int
	decoder bool
}

// helperBlock is the fixed helper text of CODEGEN.md §8.2 split into its banner and its units.
type helperBlock struct {
	head, foot string
	units      []*unit
}

var declLine = regexp.MustCompile(declPattern)

// parseUnits splits text into units: a declaration at column 0, the `/**` line above it included.
func parseUnits(lines []string, decoder bool) []*unit {
	var out []*unit
	var cur []string
	name, group, blank := "", 0, true
	flush := func() {
		if name != "" {
			out = append(out, &unit{name: name, text: strings.Join(cur, newline), group: group, decoder: decoder})
		}
		cur, name = nil, ""
	}
	for _, line := range lines {
		m := declLine.FindStringSubmatch(line)
		switch {
		case line == "":
			flush()
			blank = true
			continue
		case m != nil && name != "":
			flush()
		case strings.HasPrefix(line, docOpen) && name != "":
			flush()
		}
		if len(cur) == 0 && blank {
			group++
			blank = false
		}
		if m != nil {
			name = m[1]
		}
		cur = append(cur, line)
	}
	flush()
	return out
}

var (
	helpers    = sync.OnceValue(func() *helperBlock { return parseHelpers(helperText) })
	decodeUnit = sync.OnceValue(func() []*unit {
		return parseUnits(strings.Split(strings.TrimSuffix(decodeText, newline), newline), true)
	})
)

// parseHelpers splits the §8.2 text: three banner lines, the units, one closing banner line.
func parseHelpers(text string) *helperBlock {
	lines := strings.Split(strings.TrimSuffix(text, newline), newline)
	return &helperBlock{
		head:  strings.Join(lines[:bannerLines], newline),
		foot:  lines[len(lines)-1],
		units: parseUnits(lines[bannerLines:len(lines)-1], false),
	}
}

// closure is the units a set of used names needs, in text order: a unit is kept when its name is used or appears in a kept unit (the texts are fixed, so scanning them is exact).
func closure(units []*unit, used map[string]bool, always ...string) []*unit {
	keep := maps.Clone(used)
	for _, name := range always {
		keep[name] = true
	}
	for grown := true; grown; {
		grown = false
		for _, u := range units {
			if keep[u.name] && keepMentioned(units, u, keep) {
				grown = true
			}
		}
	}
	var out []*unit
	for _, u := range units {
		if keep[u.name] {
			out = append(out, u)
		}
	}
	return out
}

// keepMentioned keeps every unit the kept unit u names, and reports whether it kept one more.
func keepMentioned(units []*unit, u *unit, keep map[string]bool) bool {
	grown := false
	for _, o := range units {
		if !keep[o.name] && mentions(u.text, o.name) {
			keep[o.name], grown = true, true
		}
	}
	return grown
}

// mentions reports that text names name as a whole word.
func mentions(text, name string) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], name)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(name)
		if !wordAt(text, start-1) && !wordAt(text, end) {
			return true
		}
		i = end
	}
}

// wordAt reports that text[i] is an identifier character.
func wordAt(text string, i int) bool {
	if i < 0 || i >= len(text) {
		return false
	}
	c := text[i]
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// joinUnits is units written one after the other, a blank line between two that the text separates.
func joinUnits(units []*unit) string {
	var b strings.Builder
	for i, u := range units {
		if i > 0 {
			b.WriteString(newline)
			if u.group != units[i-1].group {
				b.WriteString(newline)
			}
		}
		b.WriteString(u.text)
	}
	return b.String()
}

// neededUnits are the units the file uses, helpers first then decoder helpers, each in text order.
func (g *gen) neededUnits() (helper, decoder []*unit) {
	all := append(slices.Clone(helpers().units), decodeUnit()...)
	for _, u := range closure(all, g.used, alwaysUnits...) {
		if u.decoder {
			decoder = append(decoder, u)
		} else {
			helper = append(helper, u)
		}
	}
	return helper, decoder
}

// helperSource is the helper block with the units the file uses and no others, CanonEvalError and CanonTable always (CODEGEN.md §8.2).
func (g *gen) helperSource(units []*unit) string {
	h := helpers()
	return h.head + newline + newline + joinUnits(units) + newline + h.foot
}
