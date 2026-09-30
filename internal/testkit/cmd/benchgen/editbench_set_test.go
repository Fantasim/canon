//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// benchDraws bounds the draws per accepted Set: a draw may find no fresh value (a Bool already
// flipped, an enum of one member).
const benchDraws = 20

// collections are the kinds whose elements are the entries a Set draws from.
var collections = []canon.ValueKind{canon.KindList, canon.KindKeyedList, canon.KindTable, canon.KindMap}

// setter times Sets drawn across every top-level value: an entry anywhere in a collection, a
// kind the entry holds, a field of that kind, and a fresh valid value for it (log-2026-09-29 M4
// U7b-r, U7b-r3). Kinds are field kinds: a text is split into string, asset and union.
type setter struct {
	p           *canon.Project
	base        canon.Revision
	rng         *rand.Rand
	roots       []string
	fresh       *freshValues
	edits       []time.Duration
	evals       []time.Duration
	rejects     []time.Duration
	present     map[string]bool
	drawn       map[string]bool
	accepted    map[string]int
	notEditable int
	guarded     bool // the benchmark: more than benchRejectPct rejected fails
}

// editTimes measures the view models, then benchEdits accepted Sets, each an Edit (re-check and
// write) then an Evaluate of its entry; rejected and refused Sets are counted apart.
func editTimes(t *testing.T, p *canon.Project, r *report, target benchTarget) {
	s := newSetter(t, p, r, target)
	s.run(t)
	all := len(s.edits) + len(s.rejects) + s.notEditable
	r.add(fmt.Sprintf("Edit one field, p95 of %d accepted Sets", len(s.edits)), seconds(p95(s.edits), targetEdit, true))
	r.add(fmt.Sprintf("Edit the re-check rejected, p95 of %d (%d%%)", len(s.rejects), len(s.rejects)*benchPercent/all),
		seconds(p95(s.rejects), targetEdit, false))
	r.add(fmt.Sprintf("Sets refused as not editable, %d (share %%)", s.notEditable),
		measure{value: float64(s.notEditable * benchPercent / all), limit: benchRejectPct, unit: "%", gated: s.guarded})
	r.add("Evaluate one entry after an edit, p95", seconds(p95(s.evals), targetEval, true))
	t.Logf("accepted Sets by kind %v", s.accepted)
	s.checkKinds(t)
}

// newSetter is a setter of target's project, its facts from the view models r measures.
func newSetter(t *testing.T, p *canon.Project, r *report, target benchTarget) *setter {
	facts := viewModels(t, p, r)
	rng := rand.New(rand.NewPCG(benchSeed, benchSeed))
	return &setter{
		p: p, base: p.Revision(), rng: rng, roots: facts.roots, guarded: target.guarded,
		fresh: &freshValues{
			p: p, rng: rng, dir: target.dir, enums: facts.enums, assets: facts.assets,
			keys: map[string][]string{}, files: map[string][]string{}, held: map[string]map[any]bool{},
		},
		present: map[string]bool{}, drawn: map[string]bool{}, accepted: map[string]int{},
	}
}

// checkKinds is the guard that makes a skipped kind visible: every kind present in the entries
// walked is drawn and accepted at least once; on the benchmark a miss fails, else it is reported.
func (s *setter) checkKinds(t *testing.T) {
	t.Helper()
	for _, k := range slices.Sorted(maps.Keys(s.present)) {
		var miss string
		switch {
		case !s.drawn[k]:
			miss = "a %s field was present and never drawn"
		case s.accepted[k] == 0:
			miss = "no accepted Set of a %s field, though one was drawn"
		default:
			continue
		}
		if s.guarded {
			t.Errorf(miss, k)
		} else {
			t.Logf("reported: "+miss, k)
		}
	}
}

// run draws until benchEdits Sets are accepted; too many draws without a Set fail the run, and
// on the benchmark more than benchRejectPct rejected Sets (a sampling bias). The examples' checks
// tie fields to one another by design (sums, a default per table): their share is reported.
func (s *setter) run(t *testing.T) {
	for draws := 0; len(s.edits) < benchEdits; draws++ {
		if draws > benchEdits*benchDraws {
			t.Fatalf("%d accepted Sets in %d draws", len(s.edits), draws)
		}
		if s.guarded && len(s.rejects)*(benchPercent-benchRejectPct) > benchRejectPct*benchEdits {
			t.Fatalf("%d Sets rejected for %d accepted: more than %d%% (sampling bias)", len(s.rejects), len(s.edits), benchRejectPct)
		}
		if f, ok := s.draw(t); ok {
			s.set(t, f)
		}
	}
}

// draw is a field of a random entry of a random top-level value, and its fresh value.
func (s *setter) draw(t *testing.T) (draft, bool) {
	ctx := context.Background()
	root, err := s.p.Value(ctx, s.roots[s.rng.IntN(len(s.roots))])
	if err != nil {
		t.Fatal(err)
	}
	entry := root
	if slices.Contains(collections, root.Kind) && root.Len() > 0 {
		if entry, err = root.Child("[#" + strconv.Itoa(s.rng.IntN(root.Len())) + "]"); err != nil {
			t.Fatal(err)
		}
	}
	byKind := map[string][]*canon.Value{}
	scalars(entry, byKind)
	kinds := slices.Sorted(maps.Keys(byKind))
	if len(kinds) == 0 {
		return draft{}, false
	}
	for _, k := range kinds {
		s.present[k] = true
	}
	kind := kinds[s.rng.IntN(len(kinds))]
	s.drawn[kind] = true
	v := byKind[kind][s.rng.IntN(len(byKind[kind]))]
	lit, ok := s.fresh.value(v)
	return draft{path: v.Path, entry: entry.Path, kind: kind, lit: lit}, ok
}

// draft is one Set to make: the field, the entry Evaluate shows, the field's kind, the value.
type draft struct {
	path, entry string
	kind        string
	lit         canon.Lit
}

// scalars adds the editable scalar values at or below v by field kind.
func scalars(v *canon.Value, into map[string][]*canon.Value) {
	if _, ok := freshByKind[v.Kind]; ok {
		if v.Editable.Mode != canon.EditNone {
			k := fieldKind(v)
			into[k] = append(into[k], v)
		}
		return
	}
	for _, c := range v.Children() {
		scalars(c, into)
	}
}

// set is one timed Edit, then, when the re-check accepted it, one timed Evaluate of its entry.
func (s *setter) set(t *testing.T, d draft) {
	ctx := context.Background()
	start := time.Now()
	res, err := s.p.Edit(ctx, canon.Edit{Base: s.base, Ops: []canon.Op{canon.Set(d.path, d.lit)}})
	took := time.Since(start)
	switch {
	case errors.Is(err, canon.ErrRejected):
		s.rejects = append(s.rejects, took)
		return
	case errors.Is(err, canon.ErrNotEditable):
		s.notEditable++ // Editable said yes for the value, the Set's op says no (a key field)
		return
	case err != nil || !res.Applied:
		t.Fatalf("Edit %s: %v, %+v", d.path, err, res)
	}
	s.edits, s.base = append(s.edits, took), res.Revision
	s.accepted[d.kind]++
	start = time.Now()
	if _, err := s.p.Evaluate(ctx, canon.EvalRequest{Base: s.base, Path: d.entry}); err != nil {
		t.Fatalf("Evaluate %s: %v", d.entry, err)
	}
	s.evals = append(s.evals, time.Since(start))
}
