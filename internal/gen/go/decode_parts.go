package gogen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// decodeUnit is the text one field slot or one stored fn adds to a decoder, and whether it reads the whole document (an inline variant's case decoder takes it), which a part holding it must be passed.
type decodeUnit struct {
	text string
	raw  bool
}

// decodeBody writes a record's or case's decoder: its object and key case, each field in declaration order, then each stored fn's `$` key in method order, as the C++ loader reads them. A body of more than the budget keeps the object and key case, calls decode<T>_<N> parts in order, and returns nil; each part holds whole fields (CODEGEN.md §2.11, §5.13). A decoder that binds no obj (a types-mode one whose only read fields are @json(inline) variants) is cut too: its parts take the document and the value, not the object.
func (g *gen) decodeBody(b *body) {
	defer g.enter(b.owner)()
	g.temps = 0
	g.inlineFolds(b)
	lc, fn := g.lc, g.decodeFunc(b.key)
	g.printf(funcOpenFormat, fn, lc.Name, lc.Path, lc.Raw, g.rawType(), lc.Out, b.goName)
	keys := g.bodyKeys(b)
	open := g.captured(func() { g.openObject(keys) })
	units := g.fieldUnits(b)
	bound := len(keys) > 0 // a part takes the object the key case opens, when it opens one
	ranges := g.partRanges(unitTexts(units), lineCount(open)+decodeReturnLines, decodeReturnLines)
	g.body.WriteString(open)
	if ranges == nil {
		g.body.WriteString(strings.Join(unitTexts(units), ""))
	}
	for n, r := range ranges {
		g.printf(partCallFormat, lc.Err, ir.GoPart(fn, n), strings.Join(g.partArgs(units[r.first:r.end], bound), listSep))
	}
	g.printf(returnNil)
	for n, r := range ranges {
		g.decodePart(b, ir.GoPart(fn, n), units[r.first:r.end], bound)
	}
}

// decodePart writes one part of a decoder: the same name, path, object (when the decoder binds one) and value, and the whole document when a unit reads it.
func (g *gen) decodePart(b *body, name string, units []decodeUnit, bound bool) {
	g.printf(partFuncFormat, name, strings.Join(g.partParams(b, units, bound), listSep))
	g.body.WriteString(strings.Join(unitTexts(units), ""))
	g.printf(returnNil)
}

// partParams are the parameters of a part holding units: name and path, the document when a unit reads it, the object when the decoder binds one, and the value being decoded.
func (g *gen) partParams(b *body, units []decodeUnit, bound bool) []string {
	lc, raw := g.lc, g.rawType()
	params := []string{fmt.Sprintf(partNameParams, lc.Name, lc.Path)}
	if readsDocument(units) {
		params = append(params, fmt.Sprintf(partRawParam, lc.Raw, raw))
	}
	if bound {
		params = append(params, fmt.Sprintf(partObjParam, lc.Obj, raw))
	}
	return append(params, fmt.Sprintf(partOutParam, lc.Out, b.goName))
}

// partArgs are the arguments decode<T> passes a part holding units, in partParams' order.
func (g *gen) partArgs(units []decodeUnit, bound bool) []string {
	lc := g.lc
	args := []string{lc.Name, lc.Path}
	if readsDocument(units) {
		args = append(args, lc.Raw)
	}
	if bound {
		args = append(args, lc.Obj)
	}
	return append(args, lc.Out)
}

// readsDocument reports a unit that reads the whole document.
func readsDocument(units []decodeUnit) bool {
	return slices.ContainsFunc(units, func(u decodeUnit) bool { return u.raw })
}

// unitTexts are the texts of units.
func unitTexts(units []decodeUnit) []string {
	texts := make([]string, len(units))
	for i, u := range units {
		texts[i] = u.text
	}
	return texts
}

// fieldUnits are the units of b's decoder in order: one per field slot (its read, then its define lookup), then one per stored fn's `$` key; a unit that writes nothing is left out.
func (g *gen) fieldUnits(b *body) []decodeUnit {
	var units []decodeUnit
	add := func(write func()) {
		g.readsDoc = false
		if text := g.captured(write); text != "" {
			units = append(units, decodeUnit{text, g.readsDoc})
		}
	}
	for _, s := range b.slots {
		if s.fn == nil && !s.isInput() {
			add(func() {
				g.readField(b, s)
				g.readDefine(s, g.lc.Out, g.keyLoc(strings.Join(s.src.WirePath, dot)))
			})
		}
	}
	for _, fn := range b.methods {
		add(func() { g.readMethod(b, fn) })
	}
	return units
}

// readFields reads b's fields into out, then each stored fn's `$` key.
func (g *gen) readFields(b *body) {
	g.body.WriteString(strings.Join(unitTexts(g.fieldUnits(b)), ""))
}
