package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/syntax"
)

// runExplain is `canon explain <path> [--layer …] [--depth <n>]` (CLI.md §3.7).
func runExplain(inv *invocation) int {
	if len(inv.args) != 1 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdExplain, errOneArg))
	}
	fsys := newSourceFS()
	p, err := inv.openProjectFS(fsys)
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	v, err := p.Value(inv.ctx, inv.args[0])
	var perr *canon.PathError
	if errors.Is(err, canon.ErrInputField) && errors.As(err, &perr) {
		return inv.explainInput(p, perr.Detail)
	}
	if err != nil {
		return inv.fail(err)
	}
	parts := newPartSource(inv.ctx, p)
	if inv.opt.format == formatJSON {
		return inv.explainJSON(parts, v)
	}
	x := &explainer{w: inv.env.Stdout, qualifier: qualifier(v.Path, inv.args[0]), parts: parts}
	x.src = &sources{fs: fsys, root: p.Root(), files: map[string]*syntax.File{}}
	if err := x.text(v, inv.opt.depth); err != nil {
		return inv.fail(err)
	}
	return exitOK
}

// explainJSON writes v's JSON line, its parts down to --depth.
func (inv *invocation) explainJSON(parts *partSource, v *canon.Value) int {
	obj, err := parts.object(v, inv.opt.depth)
	if err != nil {
		return inv.fail(err)
	}
	if err := inv.writeJSONLine(explainLine{Explain: obj}); err != nil {
		return inv.fail(err)
	}
	return exitOK
}

// qualifier is the `package:` prefix of a canonical path the argument did not write: a path
// prints as the argument was written, qualified only when it was.
func qualifier(canonical, arg string) string {
	i := strings.IndexByte(canonical, pkgMark)
	if i < 0 || strings.HasPrefix(arg, canonical[:i+1]) {
		return ""
	}
	return canonical[:i+1]
}

// explainInput prints an input field's canonical path and `input from env NAME` (EVALUATION.md §11.2).
func (inv *invocation) explainInput(p *canon.Project, env string) int {
	arg := inv.args[0]
	dot := strings.LastIndexByte(arg, fieldMark)
	if dot < 0 {
		return inv.fail(errNoContainer)
	}
	container, err := p.Value(inv.ctx, arg[:dot])
	if err != nil {
		return inv.fail(err)
	}
	path := container.Path + arg[dot:]
	if inv.opt.format == formatJSON {
		if err := inv.writeJSONLine(explainLine{Explain: inputObject(path, env)}); err != nil {
			return inv.fail(err)
		}
		return exitOK
	}
	writeLine(inv.env.Stdout, inputHead(path, env, qualifier(path, arg)))
	return exitOK
}

// inputHead is an input field's line: its path less the qualifier the argument did not write, and
// where its value comes from.
func inputHead(path, env, qualifier string) string {
	return fmt.Sprintf(fmtExplainHead, strings.TrimPrefix(path, qualifier), fmt.Sprintf(fmtInputFrom, env))
}

// explainer writes one explanation, quoting expressions from the sources the analysis read.
type explainer struct {
	w         io.Writer
	qualifier string
	src       *sources
	parts     *partSource
}

// text writes v's block, then each part's, depth-first, down to depth levels (-1: every part).
func (x *explainer) text(v *canon.Value, depth int) error {
	head := fmt.Sprintf(fmtExplainHead, strings.TrimPrefix(v.Path, x.qualifier), oneLine(v.Text))
	if v.Type.Expr != "" {
		head += outputIndent + v.Type.Expr
	}
	writeLine(x.w, head)
	for o := &v.Origin; o != nil; o = o.Replaced {
		x.originLines(*o)
	}
	if depth == 0 {
		return nil
	}
	ps, err := x.parts.parts(v)
	if err != nil {
		return err
	}
	for _, c := range ps {
		if err := x.part(c, depth-1); err != nil {
			return err
		}
	}
	return nil
}

// part writes one part: a value's block, or an input field's line (EVALUATION.md §11.2).
func (x *explainer) part(c part, depth int) error {
	if c.value != nil {
		return x.text(c.value, depth)
	}
	writeLine(x.w, inputHead(c.path, c.env, x.qualifier))
	return nil
}

// originLines writes `<origin>  <file>:<line>  <detail>`, columns never padded, then one
// `in <fn> (<file>:<line>)` line per stack frame (API.md F13).
func (x *explainer) originLines(o canon.Origin) {
	detail, ok := originDetails[o.Kind]
	if !ok {
		return // no provenance: a pseudo-field or an enum member
	}
	label := string(o.Kind)
	if l, renamed := originLabels[o.Kind]; renamed {
		label = l
	}
	line := outputIndent + label
	if o.File != "" {
		line += outputIndent + fmt.Sprintf(fmtFileLine, o.File, o.Line)
	}
	if d := detail(x, o); d != "" {
		line += outputIndent + oneLine(d)
	}
	writeLine(x.w, line)
	for _, f := range o.Stack {
		writeLine(x.w, fmt.Sprintf(fmtFrame, f.Fn, f.File, f.Line))
	}
	if o.MoreFrames > 0 {
		writeLine(x.w, fmt.Sprintf(fmtMoreFrames, o.MoreFrames))
	}
}

func noDetail(*explainer, canon.Origin) string { return "" }

func layerDetail(_ *explainer, o canon.Origin) string { return layerWord + o.Layer }

func pointerDetail(_ *explainer, o canon.Origin) string { return pointerMark + o.Pointer }

// rowDetail is `row <n>` from a CSV cell's pointer `/<row>/<column>` (EVALUATION.md §13).
func rowDetail(_ *explainer, o canon.Origin) string {
	row, _, _ := strings.Cut(strings.TrimPrefix(o.Pointer, pathSep), pathSep)
	if row == "" {
		return ""
	}
	return rowWord + row
}

func textDetail(_ *explainer, o canon.Origin) string { return o.Text }

// sourceDetail is the expression at the origin's span, on one line: a spread or a computation.
func sourceDetail(x *explainer, o canon.Origin) string { return x.src.expr(o) }

// oneLine is s with each line break written `\n`: explain prints one line per value and origin.
func oneLine(s string) string {
	return strings.ReplaceAll(s, lineBreak, string([]byte{escapeByte, newlineLetter}))
}
