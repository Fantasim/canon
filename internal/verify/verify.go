package verify

import (
	"context"
	"fmt"
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Evaluator is what verification needs of the evaluator: Force, MarkInvalid, Where.
type Evaluator interface {
	// Force evaluates a top-level value, here a ref's target collection; false: poisoned.
	Force(ctx context.Context, root eval.Root) (value.Value, bool)
	// MarkInvalid marks the value a soft finding is about (EVALUATION.md §7.3).
	MarkInvalid(v value.Value)
	// Invalid reports a value marked invalid, by a conversion or by a soft finding.
	Invalid(v value.Value) bool
	// Where re-runs a `where` predicate on it, at path, in its package's scope, at no step cost. ok
	// false: a hard error, which it reported with that path (API.md F1).
	Where(ctx context.Context, p *types.Predicate, it value.Value, path string) (holds, ok bool)
}

// dependents is what an Evaluator also serves to verify dependent values, each root's steps charged to it (TYPES.md §11.6).
type dependents interface {
	Verifying(ctx context.Context, root eval.Root, bag *diag.Bag) *eval.StageB
}

// Assets tells whether a file exists under an asset root, matched byte for byte (TYPES.md §13.4).
type Assets interface {
	// Exists looks name up under root, written in a file of directory from (WIRE.md §2.2, §2.3).
	Exists(root, from, name string) (display string, found bool)
}

// Verifier runs stage B (EVALUATION.md §5), from any goroutine; eval.Host.Verify uses Check.
type Verifier struct {
	ev       Evaluator
	deps     dependents
	bags     map[string]*diag.Bag
	assets   Assets
	src      *sources
	declared map[eval.Root]types.Type

	mu      sync.Mutex
	indexes map[value.Value]map[value.Key]*value.Record

	memo     entryMemo // the entries kept across snapshots, nil for none (entrymemo.go)
	replayed int
}

// Index is what verification reads of a checked program, built once; the verifiers of one run share it.
type Index struct {
	src      *sources
	declared map[eval.Root]types.Type
}

// NewIndex indexes a checked program for its verifiers.
func NewIndex(prog *check.Program) *Index {
	return (*IndexCache)(nil).Index(prog)
}

// New is the verifier of a checked program, with a bag per package; nil assets finds no file.
func New(ev Evaluator, prog *check.Program, bags map[string]*diag.Bag, assets Assets) *Verifier {
	return NewShared(NewIndex(prog), ev, bags, assets)
}

// NewShared is New over an index built once, for a verifier made per call with bags of its own.
func NewShared(ix *Index, ev Evaluator, bags map[string]*diag.Bag, assets Assets) *Verifier {
	deps, _ := ev.(dependents)
	return &Verifier{
		ev:       ev,
		deps:     deps,
		bags:     bags,
		assets:   assets,
		src:      ix.src,
		declared: ix.declared,
		indexes:  map[value.Value]map[value.Key]*value.Record{},
	}
}

// Result is the outcome of verifying one top-level value.
type Result struct {
	Valid    bool      // no finding marked a part of it invalid
	Poisoned bool      // a `where` predicate raised a hard error: the caller poisons the root (§7.1)
	Unbound  []Unbound // level-1 refs no instance bound, in walk order: the caller reports E3505
}

// Unbound is a level-1 ref left unbound (EVALUATION.md §3.4), marked invalid; eval reports it.
type Unbound struct {
	Ref  *value.Ref
	Path string
}

// Check verifies a top-level value against its declared type and hands its converted value back (EVALUATION.md §5, §4.1).
func (v *Verifier) Check(ctx context.Context, root eval.Root, val value.Value) (Result, error) {
	bag := v.bags[root.Pkg]
	if bag == nil {
		return Result{}, fmt.Errorf(fmtNoBag, ErrNoBag, root.Pkg)
	}
	w := &walker{Verifier: v, ctx: ctx, bag: bag, pkg: root.Pkg, root: root, res: Result{Valid: true}, branches: map[branchKey]branchOut{}}
	if v.deps != nil {
		w.stage = v.deps.Verifying(ctx, root, bag)
	}
	t, ok := v.declared[root]
	if !ok {
		t = val.Type()
	}
	nv := w.walk(val, t, Root(root.Name), scope{})
	if w.err != nil {
		return w.res, w.err
	}
	if nv != val && !w.res.Poisoned && w.stage != nil {
		w.stage.Replace(nv)
	}
	return w.res, nil
}

type walker struct {
	*Verifier
	ctx     context.Context
	bag     *diag.Bag
	pkg     string
	root    eval.Root
	res     Result
	stage   *eval.StageB
	err     error
	halted  bool // the step budget ran out (EVALUATION.md §12.2)
	charged int  // applications charged so far

	recording []*recorder // the instances being verified, innermost last
	rec       *entryRec   // the table entry being recorded for the memo, nil for none

	branches   map[branchKey]branchOut
	cachedOnly bool
}

// scope is where a value sits: its table entry, the env its type arguments read, its field.
type scope struct {
	entry   string
	retired bool // a retired entry encloses it (LOCK.md §4.3)
	env     *env
	field   string // the field it is given to directly, "" for an element, a key or a map value
	dep     *depSite
	direct  bool // the value is dep's own, not one of its parts
}

// flag reports a soft finding at s and marks v invalid (EVALUATION.md §7.1).
func (w *walker) flag(s Site, b *diag.Builder, v value.Value, at *Path) {
	w.report(s, b, at)
	w.invalid(v)
}

func (w *walker) invalid(v value.Value) {
	w.ev.MarkInvalid(v)
	w.res.Valid = false
	if w.rec != nil {
		w.rec.marked = append(w.rec.marked, v)
	}
}

func (w *walker) stopped() bool {
	return w.res.Poisoned || w.halted || w.err != nil || w.ctx.Err() != nil
}
