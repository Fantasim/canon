package edit

import (
	"context"
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// sessionView is the session's own view of the state a base edit plans on with no layer: its
// project, layers active, and its analysis of the state at generation gen (API.md W10).
type sessionView struct {
	project *build.Project
	snap    *Snapshot
	host    wire.Host
	gen     int
}

// newPlanner is the applier of an edit against base. A base edit while layers are active plans its
// writes, typing, defaults and Undo on the state analyzed with no layer, what the base file holds,
// and keeps the session to judge each operation's verdict (API.md W8, W10, V1; EDIT-MATERIALIZE).
func newPlanner(ctx context.Context, env Env, base *Snapshot) (*applier, error) {
	if env.EditLayer != "" || len(base.a.Layers()) == 0 {
		return newApplier(ctx, env, base), nil
	}
	bare := env
	bare.Project = env.Project.WithLayers(nil)
	if bare.Scope == nil {
		bare.Scope = base.a.Selected()
	}
	an, err := bare.Project.AnalyzeOnly(ctx, bare.Scope)
	if err != nil {
		return nil, asIO(err)
	}
	a := newApplier(ctx, bare, NewSnapshot(an))
	a.session = &sessionView{project: env.Project, snap: base, host: env.Host(base.a)}
	return a, nil
}

// sessionNow is the session's analysis of the current state, analyzed again once the files change.
func (a *applier) sessionNow() (*sessionView, error) {
	v := a.session
	if v.snap != nil && v.gen == a.gen {
		return v, nil
	}
	an, err := v.project.Over(a.overlay()).AnalyzeOnly(a.ctx, a.selected)
	if err != nil {
		return nil, asIO(err)
	}
	v.snap, v.host, v.gen = NewSnapshot(an), a.env.Host(an), a.gen
	return v, nil
}

// sessionVerdict is op's refusal as the session judges its path, layers active: the rows a layer
// decides (`layered`, a Move a layer's position names) and all others, as Editable gives them; nil
// for a path the session does not resolve, whose resolution the base plan reports (E21).
func (a *applier) sessionVerdict(op Operation, p Path) error {
	v, err := a.sessionNow()
	if err != nil {
		return err
	}
	res, ok := v.snap.resolves(p)
	if !ok {
		return a.prefixVerdict(v.snap, p, op)
	}
	j := v.snap.judge(res, op.Kind, "")
	j.ctx = a.ctx
	return a.editable(j)
}

// prefixVerdict is the `layered` refusal of the deepest prefix of p the session resolves, for a
// path below a value an active layer replaced whole, which lacks it (API.md W10: the ancestor
// rule); when none resolves, lineVerdict's; nil when not layered: the base plan judges the rest.
func (a *applier) prefixVerdict(s *Snapshot, p Path, op Operation) error {
	for n := range slices.Backward(p.Segs) {
		res, ok := s.resolves(Path{Package: p.Package, Root: p.Root, Segs: p.Segs[:n]})
		if !ok {
			continue
		}
		j := s.judge(res, op.Kind, "")
		j.ctx = a.ctx
		if layeredRow(j) == ReasonLayered {
			return &NotEditableError{Reason: ReasonLayered, Layer: j.blockedBy()}
		}
		return nil
	}
	if a.replaying {
		return nil // a root the edit leaves uncomputed is the re-check's (E19; DECISIONS 273)
	}
	return s.lineVerdict(p)
}

// lineVerdict is the `layered` refusal of an active layer with a line on, above or below p, whose
// root the session does not compute: a layer whose line is in error still states the path (W10).
func (s *Snapshot) lineVerdict(p Path) error {
	if x, ok := s.layerOnPath(p); ok {
		return &NotEditableError{Reason: ReasonLayered, Layer: x}
	}
	return nil
}

// layerOnPath is layerOn of p's root, false when s has no such let.
func (s *Snapshot) layerOnPath(p Path) (string, bool) {
	r, err := s.lookup(Path{Package: p.Package, Root: p.Root})
	if err != nil || r.enum != nil {
		return "", false
	}
	return s.layerOn(r, Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: p.Segs}.String())
}

// resolves is p read in s, false when s does not resolve it or it names an enum member.
func (s *Snapshot) resolves(p Path) (resolution, bool) {
	res, err := s.open(p)
	return res, err == nil && res.root.enum == nil
}

// sessionDefault reports v, the base's evaluated default (setsDefault), the default the session
// gives the field at path, read in the session's record with its own host; true when there is no
// session or it has no such field.
func (a *applier) sessionDefault(path string, v value.Value) bool {
	if a.session == nil {
		return true
	}
	s, err := a.sessionNow()
	p, perr := Parse(path)
	if err != nil || perr != nil {
		return true
	}
	res, err := s.snap.open(p)
	if err != nil {
		return true
	}
	rec, f, ok := fieldAt(res)
	if !ok || !hasDefault(f) {
		return true
	}
	d, ok := (&applier{ctx: a.ctx, host: s.host}).defaultOf(withField(rec, f.Index, nil), f.Index)
	return ok && d.CanonText() == v.CanonText()
}

// undoReplayed refuses a base edit whose Undo would be refused (API.md E22): when an active layer's
// line lies near an inverse, the Undo is planned as this edit is, each inverse judged by the session
// in the state the inverses before it leave; nothing is written.
func (a *applier) undoReplayed(undo []Operation) error {
	if a.session == nil || !a.layerBlockable(undo) {
		return nil
	}
	if err := a.settle(); err != nil {
		return err
	}
	over := a.overlay()
	env := a.env
	env.Project = env.Project.Over(over)
	b := newApplier(a.ctx, env, a.snap)
	b.selected = a.selected
	b.session = &sessionView{project: a.session.project.Over(over)}
	b.replaying = true
	for i, op := range undo {
		b.step = i
		err := b.operation(op)
		var ne *NotEditableError
		switch {
		case errors.As(err, &ne):
			return undoLine(op.Path, err)
		case err != nil && fatal(err):
			return err
		}
	}
	return nil
}
