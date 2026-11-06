package edit

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/eval"
)

// stuckRefs is what refuses a rename (API.md E12): the refs not editable and the paths a shared
// token names (DECISIONS 483), as `pkg:path` once each, the reason, and its layer when `layered`.
type stuckRefs struct {
	refs   []stuckRef
	shared []string
	reason Reason
	layer  string
}

// stuckRef is a ref not editable and the display path of its file, which R8 orders by.
type stuckRef struct {
	file string
	ref  Ref
}

// all is the refs not editable in R8 order, whichever analysis listed them, then the shared paths
// not among them, as `pkg:path` once each: what Detail lists (API.md R8; DECISIONS 503).
func (s stuckRefs) all() []string {
	refs := slices.Clone(s.refs)
	slices.SortStableFunc(refs, func(a, b stuckRef) int { return r8Order(a.file, a.ref, b.file, b.ref) })
	var out []string
	for _, r := range refs {
		out = appendOnce(out, r.ref.Package+packageMark+r.ref.Path)
	}
	for _, p := range s.shared {
		out = appendOnce(out, p)
	}
	return out
}

// add is s with ref r of side stuck for why, set by layer when `layered`: `layered` wins the reason (DECISIONS 464).
func (s *stuckRefs) add(side refSide, r Ref, why Reason, layer string) {
	s.refs = append(s.refs, stuckRef{file: side.snap.display(r.Span.File), ref: r})
	s.reason = blame(s.reason, why)
	s.layer = side.firstLayer(s.layer, layer)
}

// refList is a target's refs in R8 order, the computed ones (R7), and the paths a shared token
// names (refsOf).
type refList struct {
	refs     []Ref
	computed map[Ref]bool
	shared   []string
}

// refSide is an analysis a rename's refs are listed and judged on, with the edit layer its judge
// reads; session for the session's, layers active.
type refSide struct {
	a       *applier
	snap    *Snapshot
	layer   string
	session bool
}

// planSide is the analysis the edit plans on.
func (a *applier) planSide() refSide {
	return refSide{a: a, snap: a.snap, layer: a.env.EditLayer}
}

// refsFor is the refs of the entry at path on the analysis Refs reads (API.md R7), and that side:
// the session's while layers are active, the plan's otherwise or when the session lacks the path;
// a nil list when neither resolves it.
func (a *applier) refsFor(path string) (refSide, *refList, error) {
	if a.session != nil {
		v, err := a.sessionNow()
		if err != nil {
			return refSide{}, nil, err
		}
		s := refSide{a: a, snap: v.snap, session: true}
		if l, err := s.refsAt(path); err != nil || l != nil {
			return s, l, err
		}
	}
	s := a.planSide()
	l, err := s.refsAt(path)
	return s, l, err
}

// refsAt is the refs of the entry at path on s, nil when s does not resolve it.
func (s refSide) refsAt(path string) (*refList, error) {
	p, err := Parse(path)
	if err != nil {
		return nil, err
	}
	res, ok := s.snap.resolves(p)
	if !ok {
		return nil, nil
	}
	return s.snap.refsOf(s.a.ctx, res.Resolved)
}

// judge is what refuses renaming the target of l on s (API.md E12), and the keyed-list elements
// whose key field the rename moves instead (DECISIONS 310).
func (s refSide) judge(l *refList) (stuckRefs, []resolution) {
	var out stuckRefs
	var moved []resolution
	if l == nil {
		return out, nil
	}
	if len(l.shared) > 0 {
		out.reason = ReasonComputed
	}
	for _, r := range l.refs {
		if r.Kind != RefValue && r.Kind != RefKey {
			continue
		}
		why, layer := s.why(r, l)
		if el, keyed := s.snap.keyedElement(r); why == ReasonKey && keyed {
			moved = append(moved, el)
		} else if why != ReasonNone {
			out.add(s, r, why, layer)
		}
	}
	out.shared = l.shared
	return out.settled(), moved
}

// settled is s with its layer only when `layered` names one (API.md W10).
func (s stuckRefs) settled() stuckRefs {
	if s.reason != ReasonLayered {
		s.layer = ""
	}
	return s
}

// withPlan is s, the session's verdict, joined by the computed refs the plan judges stuck: a
// computed ref has no token the rename could rewrite, so either analysis refusing it refuses the
// rename (API.md E11, E12; DECISIONS 503).
func (s stuckRefs) withPlan(plan refSide, l *refList) stuckRefs {
	for _, r := range l.refs {
		if !l.computed[r] || r.Kind != RefValue && r.Kind != RefKey {
			continue
		}
		why, layer := plan.why(r, l)
		if _, keyed := plan.snap.keyedElement(r); why != ReasonNone && (why != ReasonKey || !keyed) {
			s.add(plan, r, why, layer)
		}
	}
	return s.settled()
}

// firstLayer is whichever of a and b comes first in the active layers' order, the other when one
// is "": the layer a `layered` refusal names (API.md W10).
func (s refSide) firstLayer(a, b string) string {
	order := s.snap.a.Layers()
	switch {
	case a == "":
		return b
	case b == "" || slices.Index(order, a) <= slices.Index(order, b):
		return a
	}
	return b
}

// why is why the rename cannot rewrite r, a value or key ref of l, as Set judges its value. On the
// session a computed ref is refused, `layered` when a layer sets it (DECISIONS 464, 489); a stated
// one, whose token the rename rewrites, is judged on the plan, unless only a layer holds its value.
func (s refSide) why(r Ref, l *refList) (Reason, string) {
	switch {
	case !s.session:
		return s.reason(r)
	case l.computed[r]:
		if _, keyed := s.snap.keyedElement(r); keyed {
			return ReasonKey, "" // a key field the rename rewrites (DECISIONS 310)
		}
		why, layer := s.reason(r)
		return cmp.Or(why, ReasonComputed), layer
	}
	plan := s.a.planSide()
	if _, err := plan.snap.openText(r.Package + packageMark + r.Path); err != nil {
		return ReasonNone, ""
	}
	return plan.reason(r)
}

// reason is why the value or map entry r states cannot be edited on s, ReasonNone when it can,
// and the layer that sets it when `layered`.
func (s refSide) reason(r Ref) (Reason, string) {
	res, err := s.snap.openText(r.Package + packageMark + r.Path)
	if err != nil {
		return ReasonComputed, ""
	}
	j := s.snap.judge(res, OpSet, s.layer)
	j.ctx = s.a.ctx
	if why := j.reason(); why != ReasonLayered {
		return why, ""
	}
	return ReasonLayered, j.blockedBy()
}

// sessionStuck is why, the plan's verdict on plan, judged on the session's refs with layers active, plus
// the plan's stuck computed refs (API.md R7, E12); with a value uncomputed, why if the re-check refuses
// (E19; DECISIONS 499), else, with AllowErrors, the error naming it: a partial list never passes.
func (x *opCtx) sessionStuck(why stuckRefs, plan *refList) (stuckRefs, error) {
	if x.a.session == nil {
		return why, nil
	}
	s, l, err := x.a.refsFor(x.res.Canonical)
	switch {
	case errors.Is(err, ErrNoValue) && x.a.ctx.Err() == nil && !x.a.allowErrors:
		return why, nil
	case err != nil:
		return why, x.otherRoot(err)
	}
	out, _ := s.judge(l)
	return out.withPlan(x.a.planSide(), plan), nil
}

// otherRoot is err, marking an ErrNoValue at a root other than the op path's own: one the refs
// reach, which the API names qualified (API.md R7).
func (x *opCtx) otherRoot(err error) error {
	var pe *PathError
	if !errors.As(err, &pe) || !errors.Is(err, ErrNoValue) || x.res.root.pkg == nil {
		return err
	}
	pe.Other = pe.Root != eval.Root{Pkg: x.res.root.pkg.Path, Name: x.res.root.obj.Name()}
	return err
}

// appendOnce is list with x at its end unless it holds x already.
func appendOnce(list []string, x string) []string {
	if slices.Contains(list, x) {
		return list
	}
	return append(list, x)
}

// openText resolves the path written as text.
func (s *Snapshot) openText(path string) (resolution, error) {
	p, err := Parse(path)
	if err != nil {
		return resolution{}, err
	}
	return s.open(p)
}

// gateRefs is err, a Rename's `computed` refusal by the editability gate (API.md E21), with what
// a rename past the gate would list (E12), or the root whose missing value leaves the list
// incomplete (R7); err as it is for any other refusal; the context's error once it is done.
func (a *applier) gateRefs(op Operation, p Path, err error) error {
	var ne *NotEditableError
	if !errors.As(err, &ne) || op.Kind != OpRename || ne.Reason != ReasonComputed || len(ne.Refs) > 0 {
		return err
	}
	s, l, rerr := a.refsFor(p.String())
	var pe *PathError
	out := *ne
	switch {
	case a.ctx.Err() != nil:
		return a.ctx.Err()
	case errors.Is(rerr, ErrNoValue) && errors.As(rerr, &pe):
		out.Detail = fmt.Sprintf(fmtRefsGap, Path{Package: pe.Root.Pkg, Root: pe.Root.Name})
		if ne.Detail != "" {
			out.Detail += listSep + ne.Detail // the way out comes last
		}
	case rerr != nil:
		return err
	default:
		out.Refs = a.joinedStuck(s, l, p).all()
	}
	return &out
}

// joinedStuck is what refuses renaming the target of l on s, joined, when s is the session's, by
// the computed refs the plan judges stuck, as past the gate (sessionStuck; DECISIONS 503).
func (a *applier) joinedStuck(s refSide, l *refList, p Path) stuckRefs {
	why, _ := s.judge(l)
	if !s.session {
		return why
	}
	plan := a.planSide()
	if pl, err := plan.refsAt(p.String()); err == nil && pl != nil {
		return why.withPlan(plan, pl)
	}
	return why
}
