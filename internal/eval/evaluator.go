package eval

import (
	"context"
	"regexp"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
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
	depth     int  // the live frames of every root, implicit ones included (EVALUATION.md §3.3, DECISIONS 195, 210)
	implicit  int  // the live implicit frames among depth: field defaults and where runs (DECISIONS 210)
	constant  bool // a folder's: it evaluates constant expressions only (TYPES.md §15)

	states    map[check.Object]*rootState
	roots     map[Root]*rootState
	stack     []*rootState
	completed []*rootState
	verifying bool
	flushing  bool
	queue     []*rootState

	invalid    map[value.Value]bool
	history    map[value.Value]value.Value // what a layer amendment replaced with a value (history.go)
	rebuilt    map[value.Value]value.Value // a container an amendment copied to change a descendant, to the one it copies (history.go)
	keyed      map[value.Value]map[value.Key]*value.Record
	regexps    map[string]*regexp.Regexp
	frees      map[syntax.Node][]check.Object
	colls      map[collKey]*types.Collection
	fieldColls map[*types.Field]*types.Collection
	ownedBy    map[*types.RecordType]map[*types.Collection]bool
	refTypes   map[refHold]bool // types whose values may hold a ref (prune.go)
	clean      map[cleanKey]bool
	origin     map[*value.Record]*value.Record // a copy of an owning instance, to the record it copies (instance.go)
	sites      map[*types.Field]site
	pkgs       map[string]*check.Package
	stable     []StableAmendment
	bugs       []error
	selfReads  map[*syntax.FnDecl][]syntax.Expr
	bound      map[*value.Record]map[*types.Param]value.Value // each applied record instance's arguments (params.go)
	loading    *loadSite                                      // the load being decoded, and the run forcing it (decode.go)
	reads      map[*types.Field][]int                         // the fields each default reads (Reads)
	written    map[value.Value]bool                           // literals given to dependent fields, kept as written (stageb.go)
	deps       map[types.Type]bool                            // whether a type holds a dependent type (stageb.go)
	verified   map[*value.Record]any                          // what stage B made of each instance; a vector reads its parent's, finished first (stageb.go)

	parent    *Evaluator                // a vector's evaluator reads its parent's settled values (vector.go)
	vec       *vectorState              // set on a vector's evaluator only
	aside     check.Bags                // a vector's throwaway bags while its host verifies
	recording *recorder                 // set while TestCalls runs
	testStops map[string]*diag.Bag      // the running test's stopping errors, by frame package, while Test runs
	testFiles diag.Files                // the files their bags resolve against
	causes    map[*rootState]*diag.Bag  // the hard error that poisoned a value forced while tests run
	via       map[*rootState]*rootState // a value poisoned by reading another poisoned one
	stageB    *diag.Bag                 // a stage-B where predicate's hard errors, until Poison makes them the cause
	late      late                      // the view model's reads after stage E (viewexpr.go)

	memo   *memoUse
	gens   memoGens
	retags retagLog
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
		history: map[value.Value]value.Value{},
		rebuilt: map[value.Value]value.Value{},
		keyed:   map[value.Value]map[value.Key]*value.Record{},
		regexps: map[string]*regexp.Regexp{},
		frees:   map[syntax.Node][]check.Object{},

		fieldColls: map[*types.Field]*types.Collection{},
		ownedBy:    map[*types.RecordType]map[*types.Collection]bool{},
		refTypes:   map[refHold]bool{},
		clean:      map[cleanKey]bool{},
		origin:     map[*value.Record]*value.Record{},
		sites:      map[*types.Field]site{},
		selfReads:  map[*syntax.FnDecl][]syntax.Expr{},
		bound:      map[*value.Record]map[*types.Param]value.Value{},
		reads:      map[*types.Field][]int{},
		written:    map[value.Value]bool{},
		deps:       map[types.Type]bool{},
		verified:   map[*value.Record]any{},
	}
}

// state is the state of a const or let, made on first use; a vector's evaluator shares its
// parent's settled ones and forces the others afresh (DECISIONS 204).
func (e *Evaluator) state(obj check.Object) *rootState {
	st := e.states[obj]
	if st != nil {
		return st
	}
	if e.parent != nil {
		if ps := e.parent.states[obj]; ps != nil && (ps.status == done || ps.status == poisoned) {
			return ps
		}
	}
	st = &rootState{root: Root{Pkg: obj.Pkg(), Name: obj.Name()}, obj: obj}
	e.states[obj] = st
	return st
}

// rootState is the state of a top-level value by name, nil when there is none.
func (e *Evaluator) rootState(root Root) *rootState {
	if st := e.roots[root]; st != nil {
		return st
	}
	if e.parent != nil {
		if ps := e.parent.roots[root]; ps != nil {
			return e.state(ps.obj)
		}
	}
	return nil
}

// Force evaluates a top-level value; false: poisoned, broken or out of budget (EVALUATION.md §3.1).
func (e *Evaluator) Force(ctx context.Context, root Root) (value.Value, bool) {
	st := e.rootState(root)
	if st == nil {
		return nil, false
	}
	return e.force(ctx, st, nil, nil)
}

// Settled is root's value when its forcing completed; it never evaluates and changes nothing (EVALUATION.md §3.1).
func (e *Evaluator) Settled(root Root) (value.Value, bool) {
	st := e.roots[root]
	if st == nil && e.parent != nil && e.parent.roots[root] != nil {
		st = e.parent.roots[root]
		if own := e.states[st.obj]; own != nil {
			st = own // a vector's own forcing of it (DECISIONS 204)
		}
	}
	if st == nil || st.status != done {
		return nil, false
	}
	return st.v, true
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
			e.stageB = nil // a cause is its own verification's (Poison)
			e.host.Verify(ctx, st.root, st.v)
			e.stageB = nil
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
	return e.invalid[v] || e.parent != nil && e.parent.invalid[v]
}

// Poison poisons a top-level value after the fact: stage B met a hard error in it (DECISIONS
// 147); it is no longer read, amended or traversed.
func (e *Evaluator) Poison(root Root) {
	cause := e.stageB
	e.stageB = nil
	st := e.rootState(root)
	if st == nil || e.parent != nil && e.parent.roots[root] == st {
		return // a vector poisons only what it forced
	}
	st.status, st.v = poisoned, nil
	if e.causes != nil && cause != nil && e.causes[st] == nil {
		e.causes[st] = cause // its verification's where predicate failed (EVALUATION.md §7.1)
	}
}
