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
// and before its cascades, on analyses Apply made, each on Apply's goroutine only; Verdicts is M9's memo.
type Env struct {
	Project   *build.Project
	EditLayer string
	Host      func(*build.Analysis) wire.Host
	Verdicts  Verdicts
	Scope     []string // the packages each state is analyzed again over (E17a); nil: base's selection
}

// Request is an edit's operations, applied in order to the state the previous ones left (API.md
// E1), and whether its result may hold errors (E19), which can leave an id it adds unlocked (E20).
type Request struct {
	Ops         []Operation
	AllowErrors bool
}

// Plan is an edit computed in memory: its files by path then the directories left empty, deepest
// first (N6); the values cascades dropped; its Undo; the packages owning a written file (E17), the
// written files of no package (375); files not in canonical layout (M9); ids it adds or retires (E20).
type Plan struct {
	Changes      []Change
	Dropped      []Dropped
	Undo         []Operation
	Touched      []string
	Unowned      []string
	NotCanonical []string
	Unparsed     []*UnparsedError // files it would write that do not parse, in path order (M9, E21)
	Locked       []Locked
	NameClash    string                 // a RenameName's ambiguous occurrence, which E35 refuses after the re-check
	NameEdits    map[string]NameEdit    // a RenameName's identifiers inserted and removed, by file (E35)
	writes       map[string][]writeStep // each changed file's writes, for tests of M6
	asked        asked                  // its operations, which name a refusal of its Undo (CheckUndo)
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
// left, analyzed again (E1); a refusal is an *OpError. A file not in canonical layout is normalized
// and listed in NotCanonical, one that does not parse left alone and listed in Unparsed (M9, E21).
func Apply(ctx context.Context, env Env, base *Snapshot, req Request) (*Plan, error) {
	switch {
	case env.Project == nil:
		return nil, ErrNoProject
	case env.Host == nil:
		return nil, ErrNoHost
	}
	if err := RenameRequest(req.Ops, false, env.EditLayer); err != nil {
		return nil, err
	}
	a, err := newPlanner(ctx, env, base)
	if err != nil {
		return nil, refusal(err)
	}
	a.multi, a.allowErrors = len(req.Ops) > 1, req.AllowErrors
	for i, op := range req.Ops {
		a.step = i
		if err := a.operation(op); err != nil {
			return nil, &OpError{Index: i, Path: op.Path, Err: refusal(err)}
		}
	}
	a.step = cascadeStep
	if err := a.cascade(); err != nil {
		return nil, asked{ops: req.Ops, paths: a.opPaths}.cascadeRefusal(err)
	}
	p, err := a.finish()
	if err != nil {
		return nil, refusal(err)
	}
	kept, err := a.keepLocked(p.Undo, p.Locked)
	if err != nil {
		return nil, refusal(err)
	}
	undo, err := a.verifiedUndo(kept, p.Touched)
	if err == nil {
		err = a.undoReplayed(undo)
	}
	p.asked = asked{ops: req.Ops, paths: a.opPaths}
	if err != nil {
		return nil, p.asked.undoRefusal(err)
	}
	p.Undo = undo
	return p, nil
}

// asked are an edit's operations as given and each one's canonical path, which name a refusal of
// its Undo (undoRefusal).
type asked struct {
	ops   []Operation
	paths map[int]string
}

// undoRefusal is a refusal of the Undo check named by the operation it falls on, as every refusal
// is (API.md W5): the first editing the line its Undo cannot write back, else the last.
func (q asked) undoRefusal(err error) error {
	var ne *NotEditableError
	var ul *undoLineError
	last := len(q.ops) - 1
	switch {
	case len(q.ops) == 0:
		return refusal(err)
	case errors.As(err, &ul):
		i := q.lineOp(ul.line)
		return &OpError{Index: i, Path: q.ops[i].Path, Err: refusal(ul.err), Inverse: ul.line}
	case errors.As(err, &ne):
		return &OpError{Index: last, Path: q.ops[last].Path, Err: ne}
	}
	return refusal(err)
}

// cascadeRefusal is a cascade's refusal named by the operation whose cascade it is: the first on a
// driver of the field it refuses, else on its record, else the last (API.md V1; DECISIONS 474).
func (q asked) cascadeRefusal(err error) error {
	var ce *cascadeError
	if len(q.ops) == 0 || !errors.As(err, &ce) {
		return refusal(err)
	}
	i := q.driverOp(ce.drivers)
	if i < 0 {
		i = q.lineOp(ce.record)
	}
	return &OpError{Index: i, Path: q.ops[i].Path, Err: refusal(ce.err)}
}

// driverOp is the first operation whose path is one of drivers, above or below it; -1 for none.
func (q asked) driverOp(drivers []string) int {
	for k := range q.ops {
		p, ok := q.paths[k]
		if ok && slices.ContainsFunc(drivers, func(d string) bool { return within(p, d, pathMarks, true) || within(d, p, pathMarks, true) }) {
			return k
		}
	}
	return -1
}

// lineOp is the first operation whose path is line, above or below it, else the last.
func (q asked) lineOp(line string) int {
	for k := range q.ops {
		if p, ok := q.paths[k]; ok && (within(p, line, pathMarks, true) || within(line, p, pathMarks, true)) {
			return k
		}
	}
	return len(q.ops) - 1
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
	base     *Snapshot // the state the edit started from
	selected []string
	files    map[string]*fileState // by display path
	host     wire.Host             // the host of snap's analysis
	baseHost wire.Host             // the host of base's analysis
	dirty    bool                  // files changed since snap was analyzed
	undo     [][]Operation         // each operation's inverses, in the order of the operations
	dropped  []Dropped
	owners   map[string]string // the package owning each file written, by display path (E17)
	records  []touched         // E15
	given    []givenValue      // what the operations wrote in JSON sources, for E15's held data
	outer    wire.Outer        // around the value the operation types (WIRE.md 5.9)
	names    *typeNaming
	unread   *ValueError // an inverse no operation applies (handle)
	marks    *symMarks   // what the typer knew of the symbols it made
	// cascadeUndo are the cascades' inverses, which follow every other one (log-2026-09-29 M4 U4b-r)
	cascadeUndo []Operation
	emptied     map[string]string // a directory a file left, to the package directory it stops below (N6)
	locked      []Locked
	facts       *lockFacts      // the edit's lock facts its Undo keeps (API.md E23)
	held        map[string]bool // each entry an AddEntry put in a root stable table, held by the ops after it (E23)
	kept        keptCase        // the fields the last SetCase kept for its refinements to judge (E14)
	omit        []string        // the fields a SetCase leaves out: its refinements refuse them (E14)
	step        int             // the operation being applied, cascadeStep for the cascades (M6 tests)
	pinned      []value.Value   // a base Set's value the base gives as default, and its path's records (setsDefault)
	session     *sessionView    // a base edit while layers are active: the session's view (newPlanner)
	gen         int             // how many times the files changed, for sessionNow
	replaying   bool            // undoReplayed's: the inverses of an edit, judged as planned

	multi, dependent, renamed bool           // what verifiedUndo needs of the request (noteUndo)
	allowErrors               bool           // the request's AllowErrors: an id it adds may stay unlocked (E20)
	named                     []Path         // the roots its operations name
	opPaths                   map[int]string // each operation's canonical path (undoRefusal)
	nameClash                 string         // Plan.NameClash
	nameEdits                 map[string]NameEdit
	spelled                   spellings

	added map[string]*value.Record // the entries whole-table Sets added to stable tables, which a lone Set's Undo keeps (E23)
}

// NameEdit is what a RenameName adds to a file besides renaming: the places, among the file's
// identifiers in source order, of those it removes (before) and inserts (after) (API.md E35).
type NameEdit struct {
	Removed, Added []int
}

func newApplier(ctx context.Context, env Env, base *Snapshot) *applier {
	h := env.Host(base.a)
	a := &applier{ctx: ctx, env: env, snap: base, base: base, host: h, baseHost: h, files: map[string]*fileState{}, owners: map[string]string{}, emptied: map[string]string{}, held: map[string]bool{}, marks: newSymMarks(), spelled: spellings{}, opPaths: map[int]string{}}
	a.selected = env.Scope // each op is analyzed again over the edit's scope (API.md E1, E17a)
	if a.selected == nil {
		a.selected = base.a.Selected()
	}
	return a
}

// typer types an operation's values against the current state (API.md V1).
func (a *applier) typer(pkg string) Typer {
	return Typer{Host: a.host, Pkg: pkg, Outer: a.outer, marks: a.marks, spelled: a.spelled, app: a}
}

// operation applies op to the current state: its changes are planned, the files they write
// checked for canonical layout (normalized, then planned again, when one is not), then made;
// a SetCase's kept fields are then judged (API.md E14).
func (a *applier) operation(op Operation) error {
	switch op.Kind {
	case OpSetCase:
		return a.setCase(op)
	case OpRenameName:
		return a.renameName(op)
	default:
		return a.run(op, nil)
	}
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
	a.names = nil
	p, err := Parse(op.Path)
	if err != nil {
		return nil, err
	}
	if h == nil && a.session != nil {
		if err := a.sessionVerdict(op, p); err != nil {
			return nil, err
		}
	}
	res, err := a.snap.open(p)
	switch {
	case err != nil && h == nil && poisonedRoot(err):
		return a.sourceOp(op, p, err) // repairable from its source (E19)
	case err != nil:
		return nil, err
	}
	if res.root.enum != nil {
		return a.memberOp(op, res)
	}
	j := a.judgeOp(res, op.Kind)
	if err := a.editable(j); err != nil {
		return nil, err
	}
	x := &opCtx{a: a, op: op, res: res, j: j, w: newWork()}
	a.outer, a.names = x.frameAt(len(res.Steps)).outer(), x.naming()
	if h != nil {
		err := h(x)
		x.w.undo, x.w.given = nil, nil // a cascade's write gives no field held data (E15)
		return x.w, err
	}
	if err := x.handle(); err != nil {
		return nil, err
	}
	x.noteTouched()
	a.noteUndo(x)
	return x.w, nil
}

// editable is the refusal W5 names when j's value is not editable.
func (a *applier) editable(j *judge) error {
	reason := j.reason()
	if j.starErr != nil {
		return j.starErr
	}
	switch reason {
	case ReasonNone:
		return amendmentRefusal(j)
	case ReasonComputed:
		return &NotEditableError{Reason: reason, Origin: j.computedOrigin(), Detail: j.wayOut(reason)}
	case ReasonLayered:
		return &NotEditableError{Reason: reason, Layer: j.blockedBy()}
	case ReasonFormat:
		return &NotEditableError{Reason: reason, File: j.formatFile()}
	default:
		return &NotEditableError{Reason: reason, Detail: j.wayOut(reason)}
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
		return nil, &NotEditableError{Reason: ReasonLayer, Detail: memberWay(op.Kind, res)}
	}
	return retireMember(res)
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
func (a *applier) finish() (*Plan, error) {
	p := &Plan{Dropped: a.dropped, NameClash: a.nameClash, NameEdits: a.nameEdits, writes: map[string][]writeStep{}}
	for _, display := range slices.Sorted(maps.Keys(a.files)) {
		if a.files[display].normalized {
			p.NotCanonical = append(p.NotCanonical, display)
		}
		if bad := a.files[display].bad; bad != nil {
			p.Unparsed = append(p.Unparsed, bad)
		}
	}
	p.Changes = a.netChanges(p.writes)
	for _, ops := range slices.Backward(a.undo) {
		p.Undo = append(p.Undo, ops...)
	}
	p.Undo = append(p.Undo, a.cascadeUndo...)
	dirs, err := a.removedDirs()
	if err != nil {
		return nil, err
	}
	p.Changes = append(p.Changes, dirs...)
	p.Touched, p.Unowned = a.owned(slices.Concat(p.Changes, unparsedChanges(p.Unparsed)))
	p.Locked = slices.Compact(slices.SortedFunc(slices.Values(a.locked), compareLocked))
	slices.SortStableFunc(p.Dropped, func(x, y Dropped) int { return strings.Compare(x.Path, y.Path) })
	return p, nil
}

// unparsedChanges are the files that do not parse as the changes they would have been, whose
// packages the plan touches all the same (E17).
func unparsedChanges(bad []*UnparsedError) []Change {
	out := make([]Change, 0, len(bad))
	for _, u := range bad {
		out = append(out, Change{Kind: ChangeModified, Path: u.File})
	}
	return out
}

// owned are the packages owning a file the changes write (E17) and the files they write that no
// package owns, a directory removed being no file (DECISIONS 375: J7).
func (a *applier) owned(changes []Change) (touched, unowned []string) {
	pkgs, files := map[string]bool{}, map[string]bool{}
	for _, c := range changes {
		for _, path := range []string{c.Path, c.OldPath} {
			pkg, ok := a.owners[path]
			switch {
			case ok:
				pkgs[pkg] = true
			case path != "" && c.Kind != ChangeRemovedDir:
				files[path] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(pkgs)), slices.Sorted(maps.Keys(files))
}

// noteUndo records what verifiedUndo needs of x: its root, a Rename, and, in a request of several
// operations, whether it touches a dependent field or a driver (log-2026-09-29 U-E22-r).
func (a *applier) noteUndo(x *opCtx) {
	a.named = append(a.named, x.res.root.at())
	if _, ok := a.opPaths[a.step]; !ok && a.step >= 0 {
		a.opPaths[a.step] = x.res.Canonical
	}
	a.renamed = a.renamed || x.op.Kind == OpRename
	if a.multi && !a.dependent {
		a.dependent = x.touchesDependent()
	}
}
