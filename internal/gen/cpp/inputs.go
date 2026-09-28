package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
)

// inputField is a runtime input and the record declaring it (CODEGEN.md §5.12).
type inputField struct {
	rec *ir.Record
	f   *ir.Field
}

// collectInputs lists the input fields in field declaration order, which LoadInputs reads in (CODEGEN.md §7.7); an input field is a record's (EVALUATION.md §11.1).
func (g *gen) collectInputs() {
	for _, c := range g.declared() {
		fields, _ := c.shape()
		for _, f := range fields {
			switch {
			case f.Input == nil:
			case c.rec == nil:
				g.malformed(inputOutsideRecord, c.canonName()+qnameSep+f.Name)
			default:
				g.inputs = append(g.inputs, inputField{rec: c.rec, f: f})
			}
		}
	}
	g.inputNames = g.pl.Inputs()
}

// own is `::<namespace>::`: LoadInputs and the input getters name the package's items fully qualified, so no local or member hides them (CODEGEN.md §3.5).
func (g *gen) own() string { return scopeSep + g.emit.Namespace + scopeSep }

// inputSlot is field f of rec's slot, detail::<P>Inputs::<Class>::<f_>, fully qualified (CODEGEN.md §7.7).
func (g *gen) inputSlot(rec *ir.Record, f *ir.Field) string {
	class, slot := g.pl.InputSlot(rec, f)
	return g.own() + detailPrefix + g.inputNames.Namespace + scopeSep + class + scopeSep + slot
}

// inputsLoaded is detail::<P>InputsLoaded, fully qualified (CODEGEN.md §7.7).
func (g *gen) inputsLoaded() string { return g.own() + detailPrefix + g.inputNames.Loaded }

// inputSlots declares, at the end of detail, the loaded flag and a namespace of slots per record, each slot reset to none or zero (CODEGEN.md §7.7; log-2026-09-24 "W2 runtime inputs": a failed read leaves the field unset).
func (g *gen) inputSlots() {
	if len(g.inputs) == 0 {
		return
	}
	names := g.inputNames
	g.h.linef(0, inlineFlagFormat, names.Loaded)
	g.h.printf(namespaceOpenFormat, names.Namespace)
	var open *ir.Record
	for _, in := range g.inputs {
		class, slot := g.pl.InputSlot(in.rec, in.f)
		if in.rec != open {
			if open != nil {
				g.h.printf(namespaceCloseFormat, g.pl.TypeName(open))
			}
			g.h.printf(namespaceOpenFormat, class)
			open = in.rec
		}
		g.h.linef(0, inlineSlotFormat, g.memberType(in.f.Type, in.f.Optional), slot, memberInit(in.f.Type, in.f.Optional))
	}
	g.h.printf(namespaceCloseFormat, g.pl.TypeName(open))
	g.h.printf(namespaceCloseFormat, names.Namespace)
}

// inputsDecl declares LoadInputs after the snapshot and store (CODEGEN.md §2.7 step 5, §5.12).
func (g *gen) inputsDecl() {
	if len(g.inputs) > 0 {
		g.h.printf(loadInputsDeclFormat, g.inputNames.Func)
		g.h.blank()
	}
}

// inputGetter reads the field's slot; before LoadInputs it signals E8302 with the registry's text, then returns the unset slot (CODEGEN.md §5.12).
func (g *gen) inputGetter(sc *scope, rec *ir.Record, f *ir.Field) {
	name := g.getterName(f)
	slot := g.inputSlot(rec, f)
	body := fmt.Sprintf(returnFormat, slot)
	if f.Optional && !byValue(f.Type) {
		body = fmt.Sprintf(returnPtrFormat, slot)
	}
	code, message := ir.InputGetterFailure(g.p.Name + qnameSep + rec.Name + qnameSep + f.Name)
	g.doc(1, f.Doc)
	g.fail(sc.add(name, f.Name))
	g.h.linef(1, inputGetterFormat, g.getterType(f.Type, f.Optional), name, g.inputsLoaded(), quote(code), quote(message), body)
}
