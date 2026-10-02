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
	p, err := inv.openProject()
	if err != nil {
		return inv.editFail(err, start)
	}
	defer func() { _ = p.Close() }()
	if _, err := p.Check(inv.ctx); err != nil { // load every package, so a revision means the same in the next process (API.md S3)
		return inv.editFail(err, start)
	}
	res, err := p.Edit(inv.ctx, req)
	if res != nil && errors.Is(err, canon.ErrRejected) {
		return inv.writeEdit(res, start, true)
	}
	if err != nil {
		return inv.editFail(err, start)
	}
	return inv.writeEdit(res, start, false)
}

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

// readRequest is the request file's bytes, or stdin's when none is named (CLI.md §3.15).
func (inv *invocation) readRequest() ([]byte, error) {
	src := io.Reader(strings.NewReader(""))
	if inv.env.Stdin != nil {
		src = inv.env.Stdin
	}
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
		return inv.failFindings(perr.Findings, start, exitUsage)
	case errors.Is(err, canon.ErrNoValue) && errors.As(err, &path):
		return inv.failFindings(path.Findings, start, exitErrors)
	}
	code := inv.fail(err)
	if code == exitUsage && slices.ContainsFunc(editRefusals[:], func(s error) bool { return errors.Is(err, s) }) {
		return exitErrors
	}
	return code
}

// failFindings prints findings and the summary, nothing on stderr, and returns code.
func (inv *invocation) failFindings(findings []canon.Finding, start time.Time, code int) int {
	if err := inv.writeFindings(findings, summaryOf(findings), time.Since(start)); err != nil {
		return inv.fail(err)
	}
	return code
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
func (inv *invocation) writeEdit(res *canon.EditResult, start time.Time, refused bool) int {
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
	if err := inv.writeJSONLine(editLine{Edit: body}); err != nil {
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
