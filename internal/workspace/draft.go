package workspace

import (
	"context"
	"maps"
	"reflect"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/wire"
)

// Changes is operations applied in memory to a snapshot (API.md E1, V13): the operations, the
// edit layer they write into (W11) and edit.Env's host factory.
type Changes struct {
	Ops       []edit.Operation
	EditLayer string
	Host      func(*build.Analysis) wire.Host
}

// Drafted is a draft applied in memory (API.md V13): its plan, the packages owning what it
// writes (E17), and the snapshot of the sources it gives with its analysis of every package;
// that snapshot is never published.
type Drafted struct {
	Plan     *edit.Plan
	Owners   []string
	Snapshot *Snapshot
	Analysis *build.Analysis
}

// Draft is c applied in memory to s, whose analysis of every package is base (API.md V13), shared
// by identical concurrent drafts (S8) when each operation has a JSON form to key it by.
func Draft(ctx context.Context, s *Snapshot, base *build.Analysis, c Changes) (*Drafted, error) {
	run := func(ctx context.Context) (*Drafted, error) { return s.draft(ctx, base, c) }
	key, ok := draftKey(c)
	if !ok {
		return run(ctx)
	}
	return Share(ctx, s, key, run)
}

// draftKey is the shared-read key of a draft: its edit layer and each operation's JSON form,
// only when every operation reads back from it exactly, so two drafts that differ never share
// (log-2026-09-29 M4 U5b-r).
func draftKey(c Changes) (string, bool) {
	parts := []string{c.EditLayer}
	for _, op := range c.Ops {
		text, err := op.MarshalJSON()
		if err != nil {
			return "", false
		}
		var back edit.Operation
		if back.UnmarshalJSON(text) != nil || !reflect.DeepEqual(back, op) {
			return "", false
		}
		parts = append(parts, string(text))
	}
	return Key(opDraft, nil, parts...), true
}

func (s *Snapshot) draft(ctx context.Context, base *build.Analysis, c Changes) (*Drafted, error) {
	if err := edit.RenameRequest(c.Ops, true, c.EditLayer); err != nil { // a draft is no lone request (API.md E31)
		return nil, err
	}
	plan, err := s.apply(ctx, base, c)
	if err != nil {
		return nil, err
	}
	if err := s.noOverlay(base, plan.Changes); err != nil { // the rules of Edit (API.md V13, S12)
		return nil, err
	}
	owners, aliases := s.owners(base, plan)
	after := s.planned(plan.Changes, aliases)
	a, err := after.analyze(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Drafted{Plan: plan, Owners: owners, Snapshot: after, Analysis: a}, nil
}

// apply is c computed in memory against s, whose analysis is base (API.md E1).
func (s *Snapshot) apply(ctx context.Context, base *build.Analysis, c Changes) (*edit.Plan, error) {
	env := edit.Env{Project: s.b, EditLayer: c.EditLayer, Host: c.Host, Verdicts: &s.p.verdicts}
	return edit.Apply(ctx, env, edit.NewSnapshot(base), edit.Request{Ops: c.Ops})
}

// analyze is the analysis of the packages selectors name on s (Analyze).
func (s *Snapshot) analyze(ctx context.Context, selectors []string) (*build.Analysis, error) {
	return Analyze(ctx, s, selectors)
}

// planned is s with changes made in memory (API.md E18, V13): new contents as overlays, deleted
// and renamed-away files absent, at each alias too, another name a file written is read by. It is
// never published; a writer keeps what it reads, as for s.
func (s *Snapshot) planned(changes []edit.Change, aliases map[string]string) *Snapshot {
	over := maps.Clone(s.fs.over)
	if over == nil {
		over = map[string][]byte{}
	}
	dropped := map[name]*entry{}
	mine := map[string]bool{}
	put := func(display string, data []byte) {
		if abs, ok := s.b.Abs(display); ok {
			over[abs], mine[abs] = data, true
			dropAt(dropped, abs)
		}
	}
	for _, c := range changes {
		switch c.Kind {
		case edit.ChangeDeleted:
			put(c.Path, nil)
		case edit.ChangeRenamed:
			put(c.OldPath, nil)
			put(c.Path, present(c.After))
		case edit.ChangeModified, edit.ChangeCreated:
			put(c.Path, present(c.After))
		case edit.ChangeRemovedDir: // an empty directory holds no source
		}
	}
	//canon:unordered each alias is stored under its own name
	for alias, abs := range aliases {
		if data, ok := over[abs]; ok {
			over[alias], mine[alias] = data, true
			dropAt(dropped, alias)
		}
	}
	next := s.fs.fork(over, dropped)
	next.mine = mine
	s.fs.plan(next)
	return s.p.snapshot(next)
}

// present is data as an overlay holds a file that exists: never nil.
func present(data []byte) []byte {
	return append(make([]byte, 0, len(data)), data...)
}
