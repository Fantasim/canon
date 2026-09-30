package canon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// Edit is an atomic list of operations (API.md §8.1).
type Edit struct {
	Base        Revision `json:"base"`
	Ops         []Op     `json:"ops"`
	AllowErrors bool     `json:"allowErrors,omitempty"`
	DryRun      bool     `json:"dryRun,omitempty"`
	Normalize   bool     `json:"normalize,omitempty"`
	Evaluate    []string `json:"evaluate,omitempty"`
}

// FileChange is one file an edit wrote, or with DryRun would write.
type FileChange struct {
	Path    string
	Kind    ChangeKind
	OldPath string
	Before  []byte
	After   []byte
}

// Dropped is a value removed by a cascade, in its wire form (rules E14, E15).
type Dropped struct {
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// EditResult is the result of Edit (API.md §8.1).
type EditResult struct {
	Applied  bool
	Revision Revision
	Changes  []FileChange
	Findings []Finding
	Summary  Summary
	Dropped  []Dropped
	Undo     []Op
	Eval     map[string]*EvalResult
}

// editHost is the host of an edit's values, one per analysis (edit.Env.Host).
var editHost = build.EditHost

// Edit applies e atomically, all operations or none, as the one writer (rules E1-E26, S9, W15).
// Refused for its errors, it returns its result with a *RejectedError (E19); an Evaluate path
// failing after the write returns the result, Applied, with that path's error.
func (p *Project) Edit(ctx context.Context, e Edit) (res *EditResult, err error) {
	defer recoverInternal(&err)
	ops := operations(e.Ops)
	for _, path := range e.Evaluate {
		if _, err := edit.Parse(path); err != nil {
			return nil, syntaxError(path, err, 0)
		}
	}
	req := workspace.EditRequest{
		Changes: workspace.Changes{Ops: ops, EditLayer: p.editLayer, Host: editHost},
		Base:    string(e.Base), AllowErrors: e.AllowErrors, DryRun: e.DryRun, Normalize: e.Normalize,
	}
	out, err := p.workspace().Edit(ctx, req)
	if err != nil && !errors.Is(err, workspace.ErrRejected) {
		return nil, p.editError(ctx, err)
	}
	res, rerr := p.editResult(ctx, out, e.DryRun)
	if rerr != nil {
		return nil, rerr
	}
	res.Eval, rerr = p.editEval(ctx, out, e.Evaluate, res.Revision)
	switch {
	case err != nil:
		return res, &RejectedError{Findings: errorFindings(res.Findings)}
	case rerr != nil:
		return res, rerr
	}
	return res, nil
}

// editResult is out in the API's form, its revision the snapshot after the edit when written,
// else the one it was computed on (S10); Before and After only with DryRun.
func (p *Project) editResult(ctx context.Context, out *workspace.EditOutcome, dryRun bool) (*EditResult, error) {
	res := &EditResult{
		Applied: out.Applied, Changes: fileChanges(out.Changes, dryRun),
		Findings: fromDiag(out.Checked.Files, out.Checked.List), Summary: summaryOf(out.Checked.Summary),
		Dropped: droppedOf(out.Plan.Dropped), Undo: make([]Op, 0, len(out.Plan.Undo)),
	}
	for _, op := range out.Plan.Undo {
		res.Undo = append(res.Undo, opOf(op))
	}
	at := out.Before
	if out.Applied {
		at = out.After
	}
	var err error
	res.Revision, err = p.revision(context.WithoutCancel(ctx), at) // a done ctx never drops a write's result (S10)
	return res, err
}

// fileChanges are the files changes write, in byte order of Path; a directory removed is none.
func fileChanges(changes []edit.Change, dryRun bool) []FileChange {
	out := make([]FileChange, 0, len(changes))
	for _, c := range changes {
		kind := changeKinds[c.Kind]
		if kind == "" {
			continue
		}
		fc := FileChange{Path: c.Path, Kind: kind, OldPath: c.OldPath}
		if dryRun {
			fc.Before, fc.After = slices.Clone(c.Before), slices.Clone(c.After)
		}
		out = append(out, fc)
	}
	slices.SortFunc(out, func(a, b FileChange) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// droppedOf are the values cascades removed, in the API's form (rules E14, E15).
func droppedOf(list []edit.Dropped) []Dropped {
	out := make([]Dropped, 0, len(list))
	for _, d := range list {
		out = append(out, Dropped{Path: d.Path, Value: slices.Clone(d.Value)})
	}
	return out
}

// errorFindings are the error findings of fs, in their order (rule E19).
func errorFindings(fs []Finding) []Finding {
	return slices.DeleteFunc(slices.Clone(fs), func(f Finding) bool { return f.Severity != SeverityError })
}

// editEval evaluates each of paths on the sources after the edit (rule V14): the snapshot
// published, or the edit made in memory when nothing was written; rev is the result's revision.
func (p *Project) editEval(ctx context.Context, out *workspace.EditOutcome, paths []string, rev Revision) (map[string]*EvalResult, error) {
	evals := map[string]*EvalResult{}
	if len(paths) == 0 {
		return evals, nil
	}
	for _, path := range paths {
		parsed, err := edit.Parse(path)
		if err != nil {
			return evals, syntaxError(path, err, 0)
		}
		a, err := evalAnalysis(ctx, out.After, parsed.Package, false)
		if err != nil {
			return evals, err
		}
		r, err := p.evaluateOn(ctx, evalOn{s: out.After, a: a, lang: p.lang}, path)
		if err != nil {
			return evals, err
		}
		r.Revision = rev
		evals[path] = r
	}
	return evals, nil
}
