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
	// Where re-runs a `where` predicate on it, in its package's scope, at no step cost. ok
	// false: a hard error, which it reported.
	Where(ctx context.Context, p *types.Predicate, it value.Value) (holds, ok bool)
}

// Assets tells whether a file exists under an asset root, matched byte for byte (TYPES.md §13.4).
type Assets interface {
	Exists(root, path string) bool
}

// Verifier runs stage B (EVALUATION.md §5), from any goroutine; eval.Host.Verify uses Check.
type Verifier struct {
	ev       Evaluator
	bags     map[string]*diag.Bag
	assets   Assets
	src      *sources
	declared map[eval.Root]types.Type

	mu      sync.Mutex
	indexes map[value.Value]map[value.Key]*value.Record
}

// Index is what verification reads of a checked program, built once; the verifiers of one run share it.
type Index struct {
	src      *sources
	declared map[eval.Root]types.Type
}

// NewIndex indexes a checked program for its verifiers.
func NewIndex(prog *check.Program) *Index {
	return &Index{src: indexSources(prog), declared: declaredTypes(prog)}
}

// New is the verifier of a checked program, with a bag per package; nil assets finds no file.
func New(ev Evaluator, prog *check.Program, bags map[string]*diag.Bag, assets Assets) *Verifier {
	return NewShared(NewIndex(prog), ev, bags, assets)
}

// NewShared is New over an index built once, for a verifier made per call with bags of its own.
func NewShared(ix *Index, ev Evaluator, bags map[string]*diag.Bag, assets Assets) *Verifier {
	return &Verifier{
		ev:       ev,
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

// Check verifies a top-level value against its declared type (EVALUATION.md §5).
func (v *Verifier) Check(ctx context.Context, root eval.Root, val value.Value) (Result, error) {
	bag := v.bags[root.Pkg]
	if bag == nil {
		return Result{}, fmt.Errorf(fmtNoBag, ErrNoBag, root.Pkg)
	}
	w := &walker{Verifier: v, ctx: ctx, bag: bag, pkg: root.Pkg, res: Result{Valid: true}}
	t, ok := v.declared[root]
	if !ok {
		t = val.Type()
	}
	w.walk(val, t, Root(root.Name), scope{})
	return w.res, nil
}

type walker struct {
	*Verifier
	ctx context.Context
	bag *diag.Bag
	pkg string
	res Result
}

// scope is the nearest enclosing table entry and whether a retired one encloses (LOCK.md §4.3).
type scope struct {
	entry   string
	retired bool
}

// flag reports a soft finding at s and marks v invalid (EVALUATION.md §7.1).
func (w *walker) flag(s Site, b *diag.Builder, v value.Value, at *Path) {
	s.Report(b, at, w.bag)
	w.invalid(v)
}

func (w *walker) invalid(v value.Value) {
	w.ev.MarkInvalid(v)
	w.res.Valid = false
}

func (w *walker) stopped() bool {
	return w.res.Poisoned || w.ctx.Err() != nil
}
