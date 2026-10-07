package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// runEdit is `canon edit [request.json]` (CLI.md §3.15): one edit request through Project.Edit.
func runEdit(inv *invocation) int {
	start := time.Now()
	inv.opt.format = formatJSON // CLI.md §3.15: always JSON lines, failures included
	if len(inv.args) > 1 {
		return inv.editFail(fmt.Errorf(fmtArgs, cmdEdit, errAtMostOneArg), start)
	}
	data, err := inv.readRequest()
	if err != nil {
		return inv.editFail(err, start)
	}
	req, err := inv.decodeRequest(data)
	if err != nil {
		return inv.editFail(err, start)
	}
	return inv.applyEdit(req, start, wrapEdit)
}

// applyEdit applies req, which analyses only what it can affect, and prints its object through
// wrap; its revision covers the whole static read set, so the printed base stays current for the
// next run while nothing changed (API.md E17a, S3, S4; DECISIONS 330).
func (inv *invocation) applyEdit(req canon.Edit, start time.Time, wrap func(editBody) any) int {
	p, err := inv.openProject()
	if err != nil {
		return inv.editFail(err, start)
	}
	defer func() { _ = p.Close() }()
	res, err := p.Edit(inv.ctx, req)
	if res != nil && errors.Is(err, canon.ErrRejected) {
		return inv.writeEdit(res, start, true, wrap)
	}
	if err != nil {
		return inv.editFail(err, start)
	}
	return inv.writeEdit(res, start, false, wrap)
}

// wrapEdit is canon edit's line (CLI.md §3.15).
func wrapEdit(b editBody) any { return editLine{Edit: b} }

// decodeRequest reads the request and takes its "editLayer" key (CLI.md §3.15).
func (inv *invocation) decodeRequest(data []byte) (canon.Edit, error) {
	var members map[string]json.RawMessage
	err := json.Unmarshal(data, &members)
	if err == nil && members == nil || err != nil && json.Valid(data) {
		err = errNotObject // valid JSON that is not an object: no Go type text
	}
	if err != nil {
		return canon.Edit{}, &canon.PathError{Op: -1, Err: canon.ErrBadOp, Detail: err.Error()}
	}
	if raw, ok := members[keyEditLayer]; ok {
		var layer string
		if err := json.Unmarshal(raw, &layer); err != nil {
			return canon.Edit{}, &canon.PathError{Op: -1, Err: canon.ErrBadOp, Detail: err.Error()}
		}
		if inv.opt.editLayer != "" && inv.opt.editLayer != layer {
			return canon.Edit{}, fmt.Errorf(fmtQuoted, layer, errLayerConflict)
		}
		inv.opt.editLayer = layer
		delete(members, keyEditLayer)
	}
	rest, err := json.Marshal(members)
	if err != nil {
		return canon.Edit{}, fmt.Errorf(fmtWrap, err)
	}
	var req canon.Edit
	err = req.UnmarshalJSON(rest)
	return req, err
}

// stdin is the process's standard input, empty when it has none.
func (inv *invocation) stdin() io.Reader {
	if inv.env.Stdin != nil {
		return inv.env.Stdin
	}
	return strings.NewReader("")
}

// readRequest is the request file's bytes, or stdin's when none is named (CLI.md §3.15).
func (inv *invocation) readRequest() ([]byte, error) {
	src := inv.stdin()
	if len(inv.args) == 1 {
		f, err := os.Open(inv.abs(inv.args[0]))
		if err != nil {
			return nil, fmt.Errorf(fmtArgs, inv.args[0], readCause(err))
		}
		defer func() { _ = f.Close() }()
		src = f
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf(fmtArgs, cmdEdit, errUnreadable)
	}
	return data, nil
}

// editFail reports err: its findings as JSON lines with the elapsed time for a broken project or a poisoned value, else as fail; exit 1 for the refusals of API.md §15.
func (inv *invocation) editFail(err error, start time.Time) int {
	var perr *canon.ProjectError
	var path *canon.PathError
	switch {
	case errors.As(err, &perr):
		return inv.failFindings(perr.Findings, summaryOf(perr.Findings), start, exitUsage)
	case errors.Is(err, canon.ErrNoValue) && errors.As(err, &path) && len(path.Findings) > 0:
		return inv.failFindings(path.Findings, poisonedSummary(path.Findings), start, exitErrors)
	}
	code := inv.fail(err)
	if code == exitUsage && slices.ContainsFunc(editRefusals[:], func(s error) bool { return errors.Is(err, s) }) {
		return exitErrors
	}
	return code
}

// failFindings prints findings and the summary, nothing on stderr, and returns code.
func (inv *invocation) failFindings(findings []canon.Finding, summary canon.Summary, start time.Time, code int) int {
	if err := inv.writeFindings(findings, summary, time.Since(start)); err != nil {
		return inv.fail(err)
	}
	return code
}

// poisonedSummary counts a poisoned value's findings and the distinct packages they belong to.
func poisonedSummary(findings []canon.Finding) canon.Summary {
	s := summaryOf(findings)
	packages := map[string]bool{}
	for _, f := range findings {
		packages[f.Package] = true
	}
	s.Packages = len(packages)
	return s
}

// editLine is the JSON object canon edit prints (CLI.md §3.15, DECISIONS 274).
type editLine struct {
	Edit editBody `json:"edit"`
}

// editBody is its members, in a fixed order; Undo is a request canon edit accepts back, absent when the edit was refused.
type editBody struct {
	Applied  bool            `json:"applied"`
	Revision canon.Revision  `json:"revision"`
	Changes  []editChange    `json:"changes"`
	Dropped  []canon.Dropped `json:"dropped"`
	Undo     *editUndo       `json:"undo,omitempty"`
}

// editUndo is the request that reverts the edit, under the edit layer it was made in.
type editUndo struct {
	Base      canon.Revision `json:"base"`
	EditLayer string         `json:"editLayer,omitempty"`
	Ops       []canon.Op     `json:"ops"`
}

// editChange is one file written, or with dryRun to be written.
type editChange struct {
	Kind    canon.ChangeKind `json:"kind"`
	Path    string           `json:"path"`
	OldPath string           `json:"oldPath,omitempty"`
}

// writeEdit prints the edit object, then the findings and the summary as JSON lines; exit 1 when refused or when errors were written (allowErrors).
func (inv *invocation) writeEdit(res *canon.EditResult, start time.Time, refused bool, wrap func(editBody) any) int {
	body := editBody{
		Applied: res.Applied, Revision: res.Revision, Dropped: res.Dropped,
		Changes: make([]editChange, 0, len(res.Changes)),
	}
	if refused {
		body.Dropped = []canon.Dropped{}
	} else {
		body.Undo = &editUndo{Base: res.Revision, EditLayer: inv.opt.editLayer, Ops: res.Undo}
		for _, c := range res.Changes {
			body.Changes = append(body.Changes, editChange{Kind: c.Kind, Path: c.Path, OldPath: c.OldPath})
		}
	}
	if err := inv.writeJSONLine(wrap(body)); err != nil {
		return inv.fail(err)
	}
	opts := canon.WriteOptions{JSON: true, Summary: res.Summary, Duration: time.Since(start)}
	if err := canon.WriteFindings(inv.env.Stdout, inv.shown(res.Findings), opts); err != nil {
		return inv.fail(err)
	}
	if refused || res.Summary.Errors > 0 {
		return exitErrors
	}
	return exitOK
}
