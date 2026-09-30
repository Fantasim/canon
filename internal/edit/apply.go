package edit

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// Env is what Apply needs: the project base was analyzed from (M9, E1), the edit layer (W11), and
// Host, an analysis's host (V1, E6), called for base, then after each operation that changed files
// and before its cascades, on analyses Apply made; each host is used on Apply's goroutine only.
type Env struct {
	Project   *build.Project
	EditLayer string
	Host      func(*build.Analysis) wire.Host
}

// Request is an edit's operations, applied in order to the state the previous ones left (API.md E1).
type Request struct {
	Ops []Operation
}

// Plan is an edit computed in memory: its files by path then the directories left empty, deepest
// first (N6); the values cascades dropped; its Undo; the packages owning a written file (E17);
// the files not in canonical layout before it (M9); the ids it adds or retires (E20).
type Plan struct {
	Changes      []Change
	Dropped      []Dropped
	Undo         []Operation
	Touched      []string
	NotCanonical []string
	Locked       []Locked
	writes       map[string][]writeStep // each changed file's writes, for tests of M6
}

// Locked is an id an edit adds or retires, as canon.lock names its collection (API.md E20).
type Locked struct {
	Name, Key string
}

func compareLocked(a, b Locked) int {
	return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Key, b.Key))
}

// Dropped is a value a cascade removed: its canonical path with its package, its wire form.
type Dropped struct {
	Path  string
	Value json.RawMessage
}

// Apply computes req against base in memory: each operation against the state the previous ones
// left, analyzed again (E1); a refusal is an *OpError. A file not in canonical layout is
// normalized first and listed in NotCanonical, for the caller to refuse without Normalize (M9).
func Apply(ctx context.Context, env Env, base *Snapshot, req Request) (*Plan, error) {
	switch {
	case env.Project == nil:
		return nil, ErrNoProject
	case env.Host == nil:
		return nil, ErrNoHost
	}
	a := newApplier(ctx, env, base)
	for i, op := range req.Ops {
		a.step = i
		if err := a.operation(op); err != nil {
			return nil, &OpError{Index: i, Path: op.Path, Err: refusal(err)}
		}
	}
	a.step = cascadeStep
	if err := a.cascade(); err != nil {
		return nil, refusal(err)
	}
	return a.finish(), nil
}

// refusal is err as Apply returns it: a refusal this package names or a failure to read the
// project as it is, any other failure wrapped in ErrInternal (API.md X2).
func refusal(err error) error {
	var io *ioError
	if errors.As(err, &io) {
		return err
	}
	for _, known := range refusals {
		if errors.Is(err, known) {
			return err
		}
	}
	return fmt.Errorf(fmtWrapped, ErrInternal, err)
}

// applier is one Apply: the snapshot of the current state, the files as edited so far, and
// what the operations applied so far give back.
type applier struct {
	ctx      context.Context
	env      Env
	snap     *Snapshot
	selected []string
	files    map[string]*fileState // by display path
	host     wire.Host             // the host of snap's analysis
	dirty    bool                  // files changed since snap was analyzed
	undo     [][]Operation         // each operation's inverses, in the order of the operations
	dropped  []Dropped
	owners   map[string]string // the package owning each file written, by display path (E17)
	records  []touched         // E15
	given    []givenValue      // what the operations wrote in JSON sources, for E15's held data
	// cascadeUndo are the cascades' inverses, which follow every other one (log-2026-09-29 M4 U4b-r)
	cascadeUndo []Operation
	emptied     map[string]string // a directory a file left, to the package directory it stops below (N6)
	locked      []Locked
	kept        keptCase // the fields the last SetCase kept for its refinements to judge (E14)
	omit        []string // the fields a SetCase leaves out: its refinements refuse them (E14)
	step        int      // the operation being applied, cascadeStep for the cascades (M6 tests)
}

func newApplier(ctx context.Context, env Env, base *Snapshot) *applier {
	a := &applier{ctx: ctx, env: env, snap: base, host: env.Host(base.a), files: map[string]*fileState{}, owners: map[string]string{}, emptied: map[string]string{}}
	for _, pkg := range base.pkgs {
		if base.a.Bag(pkg.Path) != nil {
			a.selected = append(a.selected, pkg.Path)
		}
	}
	return a
}

// typer types an operation's values against the current state (API.md V1).
func (a *applier) typer(pkg string) Typer {
	return Typer{Host: a.host, Pkg: pkg}
}

// operation applies op to the current state: its changes are planned, the files they write
// checked for canonical layout (normalized, then planned again, when one is not), then made;
// a SetCase's kept fields are then judged (API.md E14).
func (a *applier) operation(op Operation) error {
	if op.Kind == OpSetCase {
		return a.setCase(op)
	}
	return a.run(op, nil)
}

// run applies op by its handler, or by h, a cascade's own change: its inverse and its touched
// records are the cascade's to give (API.md E14, E15).
func (a *applier) run(op Operation, h func(*opCtx) error) error {
	for {
		if err := a.settle(); err != nil {
			return err
		}
		w, err := a.plan(op, h)
		if err != nil {
			return err
		}
		again, err := a.normalize(w.written())
		switch {
		case err != nil:
			return err
		case !again:
			return a.commit(w)
		}
	}
}

// plan resolves op's path and runs its handler (API.md E21: path, resolution, editability,
// operation, value).
func (a *applier) plan(op Operation, h func(*opCtx) error) (*work, error) {
	if h == nil && int(op.Kind) >= len(handlers) {
		return nil, ErrBadOp
	}
	p, err := Parse(op.Path)
	if err != nil {
		return nil, err
	}
	res, err := a.snap.open(p)
	if err != nil {
		return nil, err
	}
	if res.root.enum != nil {
		return a.memberOp(op, res)
	}
	j := a.snap.judge(res, op.Kind, a.env.EditLayer)
	if err := a.editable(j); err != nil {
		return nil, err
	}
	x := &opCtx{a: a, op: op, res: res, j: j, w: newWork()}
	if h != nil {
		err := h(x)
		x.w.undo = nil
		return x.w, err
	}
	if err := handlers[op.Kind](x); err != nil {
		return nil, err
	}
	x.noteTouched()
	return x.w, nil
}

// editable is the refusal W5 names when j's value is not editable.
func (a *applier) editable(j *judge) error {
	switch reason := j.reason(); reason {
	case ReasonNone:
		return nil
	case ReasonComputed:
		return &NotEditableError{Reason: reason, Origin: a.snap.origin(j.res.Target)}
	case ReasonLayered:
		return &NotEditableError{Reason: reason, Layer: j.last().layer}
	case ReasonFormat:
		return &NotEditableError{Reason: reason, File: j.hiddenFile()}
	default:
		return &NotEditableError{Reason: reason}
	}
}

// memberOp is an operation on an enum member (API.md P7a): Retire, or Unretire (E4).
func (a *applier) memberOp(op Operation, res resolution) (*work, error) {
	switch {
	case op.Kind == OpUnretire:
		return nil, ErrStableKey
	case op.Kind != OpRetire:
		return nil, ErrBadOp
	case a.env.EditLayer != "":
		return nil, &NotEditableError{Reason: ReasonLayer}
	}
	return a.retireMember(res)
}

// opCtx is one operation being planned: its resolution, its editability judge, its work.
type opCtx struct {
	a   *applier
	op  Operation
	res resolution
	j   *judge
	w   *work
	nw  value.Value // Set, SetCase: the new value at the path, whose record fields E15 compares

	keptFields []string // SetCase: the old case's fields kept whose new refinements differ (E14)
}

// finish is the plan: the files that changed, the inverses last operation first (E23).
func (a *applier) finish() *Plan {
	p := &Plan{Dropped: a.dropped, writes: map[string][]writeStep{}}
	for _, display := range slices.Sorted(maps.Keys(a.files)) {
		fs := a.files[display]
		if fs.normalized {
			p.NotCanonical = append(p.NotCanonical, display)
		}
		if c, ok := fs.change(); ok {
			p.Changes = append(p.Changes, c)
			p.writes[display] = fs.steps
		}
	}
	slices.SortFunc(p.Changes, func(x, y Change) int { return strings.Compare(x.Path, y.Path) })
	for _, ops := range slices.Backward(a.undo) {
		p.Undo = append(p.Undo, ops...)
	}
	p.Undo = append(p.Undo, a.cascadeUndo...)
	p.Changes = append(p.Changes, a.removedDirs()...)
	touched := map[string]bool{}
	for _, c := range p.Changes {
		if pkg, ok := a.owners[c.Path]; ok {
			touched[pkg] = true
		}
	}
	p.Touched = slices.Sorted(maps.Keys(touched))
	p.Locked = slices.Compact(slices.SortedFunc(slices.Values(a.locked), compareLocked))
	slices.SortStableFunc(p.Dropped, func(x, y Dropped) int { return strings.Compare(x.Path, y.Path) })
	return p
}
