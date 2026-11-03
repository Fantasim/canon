package gogen

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// captured is the text write appends to the body, taken off it.
func (g *gen) captured(write func()) string {
	outer := g.body
	g.body = bytes.Buffer{}
	write()
	text := g.body.String()
	g.body = outer
	return text
}

// lineCount is the lines of a statement's text: generated statements end with a newline, and go/format keeps their line breaks.
func lineCount(text string) int { return strings.Count(text, newline) }

// partRange is the run of statements texts[first:end] one part holds.
type partRange struct{ first, end int }

// partRanges cuts a function's body of whole statements into the runs of texts that ir.PartEnds gives for the budget g.partLines (CODEGEN.md §2.11); nil when the body is written whole. whole is the lines the uncut body holds outside its statements, part the lines each part adds inside its braces.
func (g *gen) partRanges(texts []string, whole, part int) []partRange {
	if len(texts) == 0 {
		return nil
	}
	lines := make([]int, len(texts))
	for i, text := range texts {
		lines[i] = lineCount(text)
	}
	var ranges []partRange
	first := 0
	for _, end := range ir.PartEnds(lines, g.partLines, whole, part) {
		ranges = append(ranges, partRange{first, end})
		first = end
	}
	return ranges
}

// buildFunc writes build<Last>, which makes the baked data: every container's rows, then every entry (§6.2). A body of more than the budget is cut between statements into build<Last>_<N> parts taking the data, which build<Last> calls in order (CODEGEN.md §2.11, §5.9).
func (g *gen) buildFunc(d ir.GoData) {
	units := g.buildUnits()
	ranges := g.partRanges(units, buildWholeLines, 0)
	g.printf(buildOpenFormat, d.Build, d.Type, g.data, d.Type)
	if ranges == nil {
		g.body.WriteString(strings.Join(units, ""))
	}
	for n := range ranges {
		g.printf(buildCallFormat, ir.GoPart(d.Build, n), g.data)
	}
	g.printf(buildCloseFormat, g.data)
	for n, r := range ranges {
		g.printf(buildPartOpenFormat, ir.GoPart(d.Build, n), g.data, d.Type)
		g.body.WriteString(strings.Join(units[r.first:r.end], ""))
		g.body.WriteString(closeBrace + newline)
	}
}

// buildUnits are the statements of the build in today's order: every container's allocation, then every entry (CODEGEN.md §6.2).
func (g *gen) buildUnits() []string {
	var units []string
	for _, v := range g.emitted {
		if text := g.captured(func() { g.allocate(v) }); text != "" {
			units = append(units, text)
		}
	}
	for _, v := range g.emitted {
		units = append(units, g.fill(v)...)
	}
	return units
}

// fill is the statements that write each entry into its row, or a value into its members.
func (g *gen) fill(v *ir.Value) []string {
	store := g.data + dot + g.byValue[v.Name].store
	var units []string
	if !isContainer(v) {
		for _, a := range g.assign(g.valueSlot(v), v.V) {
			units = append(units, fmt.Sprintf("%s.%s = %s\n", g.data, a.name, a.expr))
		}
		return units
	}
	rec, ok := g.sub(v.Type.Elem).Named.(*ir.Record)
	if !ok {
		return nil
	}
	for i, r := range g.entries(v) {
		row := store + dot + ir.GoRows + lbracket + strconv.Itoa(i) + rbracket
		switch {
		case v.Type.Kind != types.Table:
			units = append(units, fmt.Sprintf("%s = %s\n", pointer+store+atCall+strconv.Itoa(i)+rparen, g.recordLit(rec, r)))
		case rec.Pkg != g.p.Name: // this package's row of another package's record (CODEGEN.md §5.9)
			units = append(units, fmt.Sprintf("%s = %s\n", row, g.foreignRowLit(rec, r, g.idMember(rec, r.Ident.Key.S))))
		default:
			units = append(units, fmt.Sprintf("%s = %s\n", row, g.recordLit(rec, r)))
		}
	}
	return units
}
