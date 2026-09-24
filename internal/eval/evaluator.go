package eval

import (
	"context"
	"regexp"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Options configure an evaluator (IMPLEMENTATION-PLAN §4.8).
type Options struct {
	Budget int64    // project.budget; 0: DefaultBudget (EVALUATION.md §12.2)
	Layers []string // the active layers, in stack order (EVALUATION.md §9.1)
}

// Evaluator is Canon's one evaluator over a checked program: it forces top-level values, runs
// checks, tests and precomputations, and spends one step budget. It serves one goroutine at a
// time; its findings go to the package bags.
type Evaluator struct {
	prog  *check.Program
	info  *check.Info
	host  Host
	bags  check.Bags
	opt   Options
	index *index

	budget    int64
	steps     int64
	spent     map[charge]int64
	order     []charge
	exhausted bool
	depth     int // the live user frames of every root (EVALUATION.md §3.3, DECISIONS 195)

	states    map[check.Object]*rootState
	roots     map[Root]*rootState
	stack     []*rootState
	completed []*rootState
	verifying bool
	flushing  bool
	queue     []*rootState

	invalid    map[value.Value]bool
	keyed      map[value.Value]map[value.Key]*value.Record
	regexps    map[string]*regexp.Regexp
	frees      map[syntax.Node][]check.Object
	colls      map[collKey]*types.Collection
	fieldColls map[*types.Field]*types.Collection
	ownedBy    map[*types.RecordType]map[*types.Collection]bool
	clean      map[cleanKey]bool
	sites      map[*types.Field]site
	pkgs       map[string]*check.Package
	stable     []StableAmendment
	bugs       []error
}

// status is where a top-level value is in its evaluation.
type status uint8

// rootState is a top-level value: its declaration, status and value.
type rootState struct {
	root   Root
	obj    check.Object
	status status
	v      value.Value
}

// charge is what steps are charged to, named in its package (EVALUATION.md §12.2).
type charge struct {
	pkg, name string
}

// New is the evaluator of a checked program; host serves load and verify, bags take the findings.
func New(prog *check.Program, host Host, bags check.Bags, opt Options) *Evaluator {
	e := newEvaluator(bags, opt)
	e.prog, e.host = prog, host
	if prog == nil {
		return e
	}
	e.info = prog.Info
	e.index = buildIndex(prog)
	e.pkgs = map[string]*check.Package{}
	for _, pkg := range prog.Packages {
		e.pkgs[pkg.Path] = pkg
		for _, obj := range pkg.Decls {
			if obj.Kind() == check.ObjConst || obj.Kind() == check.ObjLet {
				st := e.state(obj)
				e.roots[st.root] = st
			}
		}
	}
	return e
}

func newEvaluator(bags check.Bags, opt Options) *Evaluator {
	budget := opt.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	return &Evaluator{
		bags: bags, opt: opt, budget: budget, index: emptyIndex(),
		spent:   map[charge]int64{},
		states:  map[check.Object]*rootState{},
		roots:   map[Root]*rootState{},
		invalid: map[value.Value]bool{},
		keyed:   map[value.Value]map[value.Key]*value.Record{},
		regexps: map[string]*regexp.Regexp{},
		frees:   map[syntax.Node][]check.Object{},

		fieldColls: map[*types.Field]*types.Collection{},
		ownedBy:    map[*types.RecordType]map[*types.Collection]bool{},
		clean:      map[cleanKey]bool{},
		sites:      map[*types.Field]site{},
	}
}

// state is the state of a const or let, made on first use.
func (e *Evaluator) state(obj check.Object) *rootState {
	st := e.states[obj]
	if st == nil {
		st = &rootState{root: Root{Pkg: obj.Pkg(), Name: obj.Name()}, obj: obj}
		e.states[obj] = st
	}
	return st
}

// Force evaluates a top-level value; false: poisoned, broken or out of budget (EVALUATION.md §3.1).
func (e *Evaluator) Force(ctx context.Context, root Root) (value.Value, bool) {
	st := e.roots[root]
	if st == nil {
		return nil, false
	}
	return e.force(ctx, st, nil, nil)
}

// BeginVerification starts stage B over the values forced so far (EVALUATION.md §1, §5).
func (e *Evaluator) BeginVerification(ctx context.Context) {
	e.verifying = true
	e.queue = append(e.queue, e.completed...)
	e.flush(ctx)
}

// flush verifies the values whose evaluation completed, once no value is being forced, so a
// value is never verified while one it may reference is still being built.
func (e *Evaluator) flush(ctx context.Context) {
	if len(e.stack) > 0 || e.flushing || e.host == nil {
		return
	}
	e.flushing = true
	for len(e.queue) > 0 && ctx.Err() == nil {
		st := e.queue[0]
		e.queue = e.queue[1:]
		if st.status == done {
			e.host.Verify(ctx, st.root, st.v)
		}
	}
	e.flushing = false
}

// MarkInvalid marks a value a soft finding is about (EVALUATION.md §7.3, DECISIONS 79).
func (e *Evaluator) MarkInvalid(v value.Value) {
	if v != nil {
		e.invalid[v] = true
	}
}

// Invalid reports a value marked by a conversion or by verification.
func (e *Evaluator) Invalid(v value.Value) bool {
	return e.invalid[v]
}

// Poison poisons a top-level value after the fact: stage B met a hard error in it (DECISIONS
// 147); it is no longer read, amended or traversed.
func (e *Evaluator) Poison(root Root) {
	if st := e.roots[root]; st != nil {
		st.status, st.v = poisoned, nil
	}
}
