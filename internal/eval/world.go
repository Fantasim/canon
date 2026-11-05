package eval

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// PackageBuilder builds the world of an `expect package` into its capture (EVALUATION.md §10.5).
type PackageBuilder interface {
	BuildPackage(ctx context.Context, w *World, capture *diag.Bag)
}

// World is the test's package built afresh for one `expect package`, its substituted lets
// holding copies of the test's values; it is discarded after its expect.
type World struct {
	ev         *Evaluator
	pkg        string
	order      []Root
	subs       map[check.Object]value.Value // the copies the substituted lets take (world_copy.go)
	poisonAt   string                       // the first poisoned value of another package the world read
	poisonRoot Root
}

// Evaluator is the world's evaluator: its own package's values, other packages' the test's.
func (w *World) Evaluator() *Evaluator {
	return w.ev
}

// Order is the package's consts and lets in forced-set order, the roots of stages A to C (EVALUATION.md §2.1).
func (w *World) Order() []Root {
	return slices.Clone(w.order)
}

// world is the world of package pkg, each let of subs holding a copy of its value, reporting
// into capture, the literals of lits those copies hold its own; nil when e's host cannot serve one
// or a value cannot be copied.
func (e *Evaluator) world(pkg string, subs map[check.Object]value.Value, capture *diag.Bag, lits *litScope) *World {
	cp := e.pkgs[pkg]
	h := newWorldHost(e.host)
	if cp == nil || h == nil {
		return nil
	}
	w := &World{pkg: pkg}
	ev := e.worldEvaluator(w, h, capture)
	copies, nodes, ok := copyInto(e, ev, subs)
	if !ok {
		return nil
	}
	w.subs = copies
	for _, obj := range cp.Decls {
		if obj.Kind() == check.ObjConst || obj.Kind() == check.ObjLet {
			st := ev.state(obj)
			ev.roots[st.root] = st
			w.order = append(w.order, st.root)
		}
	}
	w.adoptLiterals(lits, nodes)
	return w
}

// worldEvaluator is w's evaluator, a child of e: fresh states for w's package, every bag the
// capture, e's counter, depth and test log; e's program and interned caches shared.
func (e *Evaluator) worldEvaluator(w *World, h *worldHost, capture *diag.Bag) *Evaluator {
	bags := check.Bags{}
	for _, cp := range e.prog.Packages {
		bags[cp.Path] = capture
	}
	v := newEvaluator(bags, e.opt)
	v.prog, v.info, v.index, v.pkgs = e.prog, e.info, e.index, e.pkgs
	v.host, h.ev = h, v
	if h.lits != nil {
		v.lits.v = h
	}
	v.regexps, v.frees, v.colls, v.fieldColls, v.pathFields = e.regexps, e.frees, e.interned(), e.fieldColls, e.pathFields
	v.ownedBy, v.holding, v.sites, v.selfReads = e.ownedBy, e.holding, e.sites, e.selfReads
	v.counter = e.counter // EVALUATION.md §10.5 item 5: one counter, never a new one
	v.depth, v.implicit = e.depth, e.implicit
	v.testStops, v.testFiles, v.testPkg = e.testStops, e.testFiles, e.testPkg
	v.parent, v.wld, w.ev = e, w, v
	return v
}

// foreign reports pkg another package than a world's own: its values are its parent's.
func (e *Evaluator) foreign(pkg string) bool {
	return e.wld != nil && pkg != e.wld.pkg
}

// forceParent forces st, another package's value, in the world's parent at the world's depth (EVALUATION.md §3.3, §10.5 item 2).
func (e *Evaluator) forceParent(ctx context.Context, st *rootState) (value.Value, bool) {
	p := e.parent
	depth, implicit := p.depth, p.implicit
	p.depth, p.implicit = e.depth, e.implicit
	defer func() { p.depth, p.implicit = depth, implicit }()
	return p.force(ctx, st, nil, nil)
}

// notePoisoned keeps the first poisoned value of another package a world read, which fails its expect (EVALUATION.md §10.5 item 4).
func (w *World) notePoisoned(pkg, name string) {
	if pkg != w.pkg && w.poisonAt == "" {
		w.poisonAt, w.poisonRoot = qualify(w.pkg, pkg, name), Root{Pkg: pkg, Name: name}
	}
}

// substituted is a substituted let's value in a world: its copy converted at the let's storage
// point and adopted as its collection, with no initializer, entry or layer.
func (r *run) substituted(st *rootState, d *syntax.LetDecl, v value.Value) value.Value {
	obj := st.obj
	if v = r.recheckCopy(v, obj.Type(), rootPath(obj.Name())); v == nil {
		return nil
	}
	v = r.store(v, obj.Type(), declSite(obj.File(), d.Name, d.Type), rootPath(obj.Name()))
	if coll := r.ev.letColl(obj); v != nil && coll != nil {
		v = r.ev.adopt(v, coll, nil)
	}
	return v
}

// letOf is the value of a let: in a world, a substituted let's copy, else its initializer's.
func (r *run) letOf(st *rootState, d *syntax.LetDecl) value.Value {
	if w := r.ev.wld; w != nil {
		if v, ok := w.subs[st.obj]; ok {
			return r.substituted(st, d, v)
		}
	}
	return r.letValue(st, d)
}

// execPackage runs `expect package [{ … }]`: its values, then its world, judged on the capture (EVALUATION.md §10.5).
func execPackage(r *run, x *syntax.ExpectStmt) flow {
	if r.ev.recording != nil { // item 6: computing vectors builds no package subject and spends nothing on it
		r.test.out.Expects = append(r.test.out.Expects, Expect{Stmt: x, Passed: true})
		return flowNext
	}
	b := r.packageSubject(x)
	if r.ev.halted(r.charge.pkg) { // item 4: E4401 in its values or its world stops the test
		r.failed = true
		return flowAbort
	}
	ex := b.ex
	ex.Passed = ex.Poisoned == "" && !b.internal && judge(x, nil, b.aborted, ex.Captured)
	if !ex.Passed {
		r.test.out.Failed = true
	}
	r.test.out.Expects = append(r.test.out.Expects, ex)
	return flowNext
}

// builtPackage is a package subject once built: the expect so far, whether evaluating its
// values aborted, and whether an internal error left its world unbuilt.
type builtPackage struct {
	ex       Expect
	aborted  bool
	internal bool
}

// packageSubject evaluates the subject's values into one capture, unlimited and deduplicated,
// then builds its world into it unless a value aborted.
func (r *run) packageSubject(x *syntax.ExpectStmt) builtPackage {
	capture := diag.NewBag(r.ev.files(), r.fr.pkg)
	capture.Truncate(math.MaxInt)
	lits := r.testScope()
	subs, b := r.substitutions(x, capture, lits)
	if !b.aborted {
		r.buildWorld(subs, capture, &b, lits)
	}
	b.ex.Captured = capture.Findings()
	return b
}

// substitutions evaluates the subject's values left to right, as a value subject is, each as
// its let's initializer is: at its type, path and collection, its kept literals into lits; the
// first hard error stops them.
func (r *run) substitutions(x *syntax.ExpectStmt, capture *diag.Bag, lits *litScope) (map[check.Object]value.Value, builtPackage) {
	saved, outer := r.sink, r.lits
	r.sink, r.poisonAt, r.lits = capture, "", lits
	defer func() { r.lits = outer }()
	subs := map[check.Object]value.Value{}
	internal := false
	for _, it := range x.Package.Items {
		obj := r.ev.info.NameUses[it.Name]
		var v value.Value
		if obj != nil && obj.Kind() == check.ObjLet { // the checker resolves every target of a test that runs (E5006, E2102)
			v = r.substitution(obj, it.Value)
		}
		if r.failed {
			break
		}
		if v == nil { // a value evaluates, or its run aborts
			internal = true
			r.bug(it)
			break
		}
		subs[obj] = v
	}
	b := builtPackage{aborted: r.failed, internal: internal, ex: Expect{Stmt: x, Poisoned: r.poisonAt, PoisonRoot: r.poisonRoot}}
	r.failed, r.sink = false, saved
	r.poisonAt, r.poisonRoot, r.poisonSpan = "", Root{}, source.Span{}
	return subs, b
}

// substitution evaluates the value of let obj as its initializer is evaluated (root.go letValue).
func (r *run) substitution(obj check.Object, e syntax.Expr) value.Value {
	at := rootPath(obj.Name())
	coll, dep, sub := r.coll, r.dep, r.sub
	if c := r.ev.letColl(obj); c != nil {
		r.coll = &collHint{c: c, at: at}
	}
	r.dep, r.sub = &depCtx{field: obj.Type(), at: e}, obj
	v := r.evalAt(e, at)
	r.coll, r.dep, r.sub = coll, dep, sub
	return v
}

// judgeSubstitutions judges the substitutions' unstored literals; a stop is b's (EVALUATION.md §10.5 item 1, DECISIONS 426).
func (r *run) judgeSubstitutions(b *builtPackage, capture *diag.Bag, lits *litScope) bool {
	sb, restore := r.reading(capture)
	defer restore()
	ok := r.judgeInto(lits, capture)
	b.ex.Poisoned, b.ex.PoisonRoot = sb.poisonAt, sb.poisonRoot // a poisoned substitution built nothing to judge
	b.aborted = !ok || sb.poisonAt != ""
	return !b.aborted
}

// buildWorld builds the world of subs through the test's PackageBuilder into capture; a Builder
// that is none, or a world e cannot make, is an internal error that fails the expect. The kept
// literals of lits no copy holds are test expressions, judged first: a hard error builds nothing.
func (r *run) buildWorld(subs map[check.Object]value.Value, capture *diag.Bag, b *builtPackage, lits *litScope) {
	pb, ok := r.test.builder.(PackageBuilder)
	var w *World
	if ok {
		w = r.ev.world(r.charge.pkg, subs, capture, lits)
	}
	if w == nil {
		b.internal = true
		r.ev.bugs = append(r.ev.bugs, fmt.Errorf(fmtWrap, errNoWorld, r.charge.name))
		return
	}
	if !r.judgeSubstitutions(b, capture, lits) {
		return
	}
	pb.BuildPackage(r.ctx, w, capture)
	r.ev.bugs = append(r.ev.bugs, w.ev.bugs...)
	if w.poisonAt != "" && b.ex.Poisoned == "" {
		b.ex.Poisoned, b.ex.PoisonRoot = w.poisonAt, w.poisonRoot
	}
}
