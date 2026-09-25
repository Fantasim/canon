package cppgen

import (
	"embed"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// inputHelperTexts are CODEGEN.md §7.7's helpers, one file per helper named after it.
var (
	//go:embed text/input
	inputHelperTexts embed.FS
)

// inputCheck is one refusal of an input's read, in order: its C++ condition and its reason.
type inputCheck struct {
	cond   string
	reason ir.InputReason
}

// inputRead is how one input field is read: its local's declarations, its refusals, the slot's value.
type inputRead struct {
	decls  []string
	checks []inputCheck
	stored string
}

// loadInputs writes the helpers the package's inputs use, in the plan's order, then LoadInputs, which resets every slot, reads the variables in field declaration order and appends one line per failure (CODEGEN.md §5.12, §7.7).
func (g *gen) loadInputs() {
	if len(g.inputs) == 0 {
		return
	}
	names := g.inputNames
	if len(names.Helpers) == 0 {
		g.malformed(inputHelperUnknown, g.p.Name)
		return
	}
	g.c.line(anonOpen)
	for _, h := range names.Helpers {
		text, err := inputHelperTexts.ReadFile(path.Join(inputHelperDir, h+inputHelperExt))
		if err != nil {
			g.malformed(inputHelperUnknown, h)
			continue
		}
		g.c.write(strings.ReplaceAll(string(text), durationLimitMark, strconv.FormatInt(types.DurationLimit, decimalBase)))
	}
	g.c.line(anonClose)
	g.c.blank()
	g.c.printf(loadInputsOpenFormat, names.Func)
	g.c.linef(1, assignFormat, g.inputsLoaded(), strconv.FormatBool(true))
	for _, in := range g.inputs {
		g.c.linef(1, assignFormat, g.inputSlot(in.rec, in.f), initBraces)
	}
	g.c.linef(1, localFormat, cppString, problemsVar, "")
	g.c.linef(1, localFormat, cppString, rawVar, "")
	for _, in := range g.inputs {
		leave := g.enter(in.rec.Name + qnameSep + in.f.Name)
		g.inputBlock(in)
		leave()
	}
	g.c.linef(1, assignFormat, errorParam, problemsVar)
	g.c.linef(1, returnFormat, problemsEmpty)
	g.c.line(closeBrace)
	g.c.blank()
}

// inputBlock reads one variable: unset or empty is unset, a failure when the field is required (EVALUATION.md §11.3 step 1).
func (g *gen) inputBlock(in inputField) {
	env := in.f.Input.Env
	g.c.linef(1, envTextFormat, g.inputNames.Helpers[0], quote(env))
	g.inputChain(in, g.inputRead(in.f))
	if !in.f.Optional {
		g.c.linef(1, elseOpen)
		g.problem(depthTwo, env, ir.InputNotSet, in.f.Type)
	}
	g.c.linef(1, closeBrace)
}

// inputChain writes the read: each refusal in order, else the slot's value (EVALUATION.md §11.3 steps 2 and 3).
func (g *gen) inputChain(in inputField, r inputRead) {
	for _, d := range r.decls {
		g.c.lineAt(depthTwo, d)
	}
	for i, c := range r.checks {
		if i == 0 {
			g.c.linef(depthTwo, ifOpenFormat, c.cond)
		} else {
			g.c.linef(depthTwo, elseIfFormat, c.cond)
		}
		g.problem(depthThree, in.f.Input.Env, c.reason, in.f.Type)
	}
	g.c.linef(depthTwo, elseOpen)
	g.c.linef(depthThree, assignFormat, g.inputSlot(in.rec, in.f), r.stored)
	g.c.linef(depthTwo, closeBrace)
}

// problem appends the failure line `<ENV>: <reason>` of ir's vocabulary (CODEGEN.md §5.12).
func (g *gen) problem(depth int, env string, reason ir.InputReason, t ir.TypeRef) {
	text := ir.InputReasonText(reason, t)
	if text == "" {
		g.malformed(kindText(t.Kind), g.at)
		return
	}
	g.c.linef(depth, appendFormat, problemsVar, quote(ir.InputLine(env, text)+newline))
}

// inputRead dispatches on the input's type (EVALUATION.md §11.1, §11.3).
func (g *gen) inputRead(f *ir.Field) inputRead {
	if read, ok := inputReads[f.Type.Kind]; ok {
		return read(g, f)
	}
	g.malformed(kindText(f.Type.Kind), g.at)
	return inputRead{}
}

// parsed is the declaration of val and its parse by the plan's §7.7 parser.
func (g *gen) parsed(f *ir.Field, local string) inputRead {
	parser := g.pl.InputParser(f.Type)
	if parser == "" {
		g.malformed(kindText(f.Type.Kind), g.at)
	}
	return inputRead{
		decls:  []string{fmt.Sprintf(localFormat, local, valVar, initBraces)},
		checks: []inputCheck{{cond: fmt.Sprintf(notParsedFormat, parser, rawVar, valVar), reason: ir.InputNotValid}},
		stored: valVar,
	}
}
