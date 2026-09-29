package cli

import (
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

// runRefs is `canon refs <path>` (CLI.md §3.8): every place that references the entry or member.
func runRefs(inv *invocation) int {
	if len(inv.args) != 1 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdRefs, errOneArg))
	}
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	res, err := p.Refs(inv.ctx, inv.args[0])
	if err != nil {
		return inv.fail(err)
	}
	if inv.opt.format == formatJSON {
		err = inv.writeRefsJSON(res)
	} else {
		inv.writeRefsText(res)
	}
	if err != nil {
		return inv.fail(err)
	}
	return exitOK
}

// writeRefsText writes the count line, then one line per reference in the API's order (API.md R8).
func (inv *invocation) writeRefsText(res *canon.RefsResult) {
	noun := wordTimes
	if len(res.Refs) == 1 {
		noun = wordTime
	}
	writeLine(inv.env.Stdout, fmt.Sprintf(fmtRefsHead, res.Target, len(res.Refs), noun))
	for _, r := range res.Refs {
		what := r.Package
		if r.Kind == canon.RefValue || r.Kind == canon.RefKey {
			what = fmt.Sprintf(fmtQualified, r.Package, r.Path)
		}
		writeLine(inv.env.Stdout, outputIndent+fmt.Sprintf(fmtRefLine, r.File, r.Line, r.Kind, what))
	}
}

// refLine is one reference's JSON line, its path written for a value or key only (CLI.md §3.8, IMPLEMENTATION-PLAN.md §8.1).
type refLine struct {
	Ref struct {
		Kind    canon.RefKind `json:"kind"`
		Package string        `json:"package"`
		Path    string        `json:"path,omitempty"`
		File    string        `json:"file"`
		Line    int           `json:"line"`
		Col     int           `json:"col"`
	} `json:"ref"`
}

// refsSummary is the JSON summary line of refs.
type refsSummary struct {
	Summary struct {
		Target string `json:"target"`
		Count  int    `json:"count"`
	} `json:"summary"`
}

// writeRefsJSON writes one line per reference, then the summary.
func (inv *invocation) writeRefsJSON(res *canon.RefsResult) error {
	for _, r := range res.Refs {
		var l refLine
		l.Ref.Kind, l.Ref.Package, l.Ref.Path = r.Kind, r.Package, r.Path
		l.Ref.File, l.Ref.Line, l.Ref.Col = r.File, r.Line, r.Col
		if err := inv.writeJSONLine(l); err != nil {
			return err
		}
	}
	var s refsSummary
	s.Summary.Target, s.Summary.Count = res.Target, len(res.Refs)
	return inv.writeJSONLine(s)
}
