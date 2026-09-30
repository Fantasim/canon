package canon_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// stressDuration is how long TestStress runs: 2 s under make check, 60 s under make stress.
var stressDuration = flag.Duration("stress.duration", 2*time.Second, "how long TestStress runs (make stress: 60s)")

// The stress project is examples/features/retirement alone, on the OS's file system, under
// /var/tmp where it exists (a disk, not a tmpfs): 8 readers, 1 editor and 1 watcher.
const (
	stressReaders = 8
	stressPkg     = "features.retirement"
	stressSource  = "../examples/features/retirement/retirement.canon"
	stressProject = "project stress {\n  canon: \"0.1\"\n}\n"
	stressTmp     = "/var/tmp"
	stressSeed    = 7
	stressHeldOne = 8                // a reader replaces the value it holds one read in this many (API.md R4)
	stressEvalOne = 4                // the editor asks Edit.Evaluate one edit in this many (API.md V14)
	stressFloor   = 10               // edits a run must commit at least
	stressCap     = 30 * time.Second // how long a run may go on past its duration to reach the floor
	labelsPrefix  = "labels "
	labelsSep     = "|"
)

// stressExtra is added to the copied example so that every read shows the labels: a title for
// Evaluate, a warning naming the three for Check.
const stressExtra = `
view Tier {
  title "{label}"
}

warn tiers.common.label == ""
  else "labels {tiers.common.label}|{tiers.rare.label}|{tiers.legendary.label}"
`

// stressTiers are the entries whose label the editor sets.
var stressTiers = []string{"common", "rare", "legendary"}

func labelPath(tier int) string { return stressPkg + ":tiers." + stressTiers[tier] + ".label" }

const tiersPath = stressPkg + ":tiers"

func entryPath(tier int) string { return stressPkg + ":tiers." + stressTiers[tier] }

// stressState is the project after the first j edits: each tier's label, and the revision the
// j-th Edit returned (state 0: the revision of the first Check).
type stressState struct {
	labels []string
	rev    canon.Revision
}

// stressClock ends a run: its duration is a minimum, the run going on until the work floor is
// met (stressFloor edits committed, every reader having read after one returned), and never
// past its cap (log-2026-09-29 M4 U7b-r2).
type stressClock struct {
	start    time.Time
	min, cap time.Time
	log      *stressLog
	readers  atomic.Int32 // readers that have read after an edit returned
}

func newStressClock(log *stressLog) *stressClock {
	start := time.Now()
	return &stressClock{start: start, min: start.Add(*stressDuration), cap: start.Add(*stressDuration + stressCap), log: log}
}

// more is whether the run goes on.
func (c *stressClock) more(ctx context.Context) bool {
	now := time.Now()
	floor := c.log.committed.Load() >= stressFloor && c.readers.Load() == stressReaders
	return ctx.Err() == nil && now.Before(c.cap) && (now.Before(c.min) || !floor)
}

// stressLog is what the editor did: every state, the next one written before its Edit starts,
// and how many Edits have returned.
type stressLog struct {
	mu        sync.Mutex
	states    []stressState
	committed atomic.Int64
}

// observation is one read: the edits returned before it started (lo) and, after it returned,
// plus the one then in flight (hi); the revision it reported, if any, and the labels it read
// by tier ("" for a tier it did not read).
type observation struct {
	lo, hi int
	rev    canon.Revision
	labels []string
}

// API.md S1, S10, R4, W13, W15 (M4 acceptance item 6): no read begun after an Edit returned
// sees an older state, a reader never goes back, a held Value never changes, and the watcher
// reports each edit once, as an edit, in revision order, with no gap.
func TestStress(t *testing.T) {
	p := openStress(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first, err := p.Check(ctx)
	if err != nil || first.HasErrors() {
		t.Fatalf("Check: %v, %+v", err, first)
	}
	log := &stressLog{states: []stressState{{labels: initialLabels(t, p), rev: first.Revision}}}
	w := &stressWatch{}
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	if err := p.Watch(watchCtx, w.event); err != nil {
		t.Fatal(err)
	}
	clock := newStressClock(log)
	readers := make([]*stressReader, stressReaders)
	done := make(chan error, stressReaders+1)
	for i := range readers {
		readers[i] = &stressReader{p: p, log: log, clock: clock, rng: rand.New(rand.NewPCG(stressSeed, uint64(i)))}
		safego.Go(func() error { return readers[i].run(ctx) }, func(err error) { done <- err })
	}
	safego.Go(func() error { return log.edit(ctx, p, clock) }, func(err error) { done <- err })
	for range stressReaders + 1 {
		if err := <-done; err != nil {
			t.Error(err)
			cancel()
		}
	}
	ran := time.Since(clock.start).Round(time.Millisecond)
	if t.Failed() {
		return
	}
	index, err := log.index()
	if err != nil {
		t.Fatal(err)
	}
	checkReaders(t, log, index, readers)
	n := int(log.committed.Load())
	events := w.wait(log.states[n].rev)
	stopWatch()
	if err := checkEvents(index, events, n, w.overlap.Load()); err != nil {
		t.Error(err)
	}
	t.Logf("%d edits, %d reads, %d events in %s (at least %s)", n, reads(readers), len(events), ran, *stressDuration)
}

// checkReaders is the work floor (log-2026-09-29 M4 U7b-r: at least stressFloor edits, each
// reader reading after one returned), then each reader's observations placed on the states.
func checkReaders(t *testing.T, log *stressLog, index map[canon.Revision]int, readers []*stressReader) {
	t.Helper()
	if n := int(log.committed.Load()); n < stressFloor {
		t.Errorf("%d edits committed when the run reached its cap, want at least %d", n, stressFloor)
	}
	for i, r := range readers {
		if !slices.ContainsFunc(r.obs, func(o observation) bool { return o.lo >= 1 }) {
			t.Errorf("reader %d never read after an edit returned, and the run reached its cap", i)
		}
		if err := log.validate(index, r.obs); err != nil {
			t.Errorf("reader %d: %v", i, err)
		}
	}
}

// openStress opens a copy of the retirement example in a new directory of the OS.
func openStress(t *testing.T) *canon.Project {
	t.Helper()
	base := ""
	if info, err := os.Stat(stressTmp); err == nil && info.IsDir() {
		base = stressTmp
	}
	dir, err := os.MkdirTemp(base, "canon-stress-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	src, err := os.ReadFile(stressSource)
	if err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(dir, "features", "retirement")
	if err := os.MkdirAll(pkgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(
		os.WriteFile(filepath.Join(dir, "project.canon"), []byte(stressProject), 0o600),
		os.WriteFile(filepath.Join(pkgDir, "retirement.canon"), append(src, stressExtra...), 0o600),
	); err != nil {
		t.Fatal(err)
	}
	p, err := canon.Open(dir, canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func initialLabels(t *testing.T, p *canon.Project) []string {
	t.Helper()
	v, err := p.Value(context.Background(), tiersPath)
	if err != nil {
		t.Fatal(err)
	}
	labels, err := tierLabels(v)
	if err != nil {
		t.Fatal(err)
	}
	return labels
}

// tierLabels are the labels of the tiers value, by tier.
func tierLabels(v *canon.Value) ([]string, error) {
	labels := make([]string, len(stressTiers))
	for i, tier := range stressTiers {
		e, err := v.Child("." + tier)
		if err != nil {
			return nil, err
		}
		l, err := e.Child(".label")
		if err != nil {
			return nil, err
		}
		if labels[i], _ = l.Str(); labels[i] == "" {
			return nil, fmt.Errorf("%s: label %s, not a string", e.Path, l.Text)
		}
	}
	return labels, nil
}

// edit sets a random tier's label to a label no edit wrote before, while the run goes on; each
// Edit's Base is the revision the previous one returned, which S5 never finds stale.
func (l *stressLog) edit(ctx context.Context, p *canon.Project, clock *stressClock) error {
	rng := rand.New(rand.NewPCG(stressSeed, stressReaders))
	for j := 1; clock.more(ctx); j++ {
		tier, label := rng.IntN(len(stressTiers)), "s"+strconv.Itoa(j)
		l.mu.Lock()
		prev := l.states[j-1]
		next := stressState{labels: slices.Clone(prev.labels)}
		next.labels[tier] = label
		l.states = append(l.states, next)
		l.mu.Unlock()
		e := canon.Edit{Base: prev.rev, Ops: []canon.Op{canon.Set(labelPath(tier), canon.Str(label))}}
		if rng.IntN(stressEvalOne) == 0 {
			e.Evaluate = []string{entryPath(tier)}
		}
		res, err := p.Edit(ctx, e)
		if err != nil {
			return fmt.Errorf("edit %d: %w", j, err)
		}
		if !res.Applied || res.Summary.Errors > 0 {
			return fmt.Errorf("edit %d: applied %v, %d errors", j, res.Applied, res.Summary.Errors)
		}
		if ev := res.Eval[entryPath(tier)]; len(e.Evaluate) > 0 && (ev == nil || ev.Revision != res.Revision) {
			return fmt.Errorf("edit %d: Edit.Evaluate %+v, want revision %s (API.md V14)", j, ev, res.Revision)
		}
		l.mu.Lock()
		l.states[j].rev = res.Revision
		l.mu.Unlock()
		l.committed.Store(int64(j))
	}
	return nil
}

// index maps each state's revision to the state: every state's revision differs, since every
// edit writes a label no other state holds.
func (l *stressLog) index() (map[canon.Revision]int, error) {
	out := make(map[canon.Revision]int, len(l.states))
	for j, s := range l.states {
		if s.rev == "" {
			continue // an Edit that failed: the test failed already
		}
		if k, dup := out[s.rev]; dup {
			return nil, fmt.Errorf("states %d and %d share revision %s", k, j, s.rev)
		}
		out[s.rev] = j
	}
	return out, nil
}

// validate places a reader's observations on states lo to hi, never going back: the earliest
// state each can be, at or after the one before it (API.md S1, S10: no stale read).
func (l *stressLog) validate(index map[canon.Revision]int, obs []observation) error {
	floor := 0
	for i, o := range obs {
		lo, hi := max(o.lo, floor), min(o.hi, len(l.states)-1)
		j := l.place(index, o, lo, hi)
		if j < 0 {
			return fmt.Errorf("read %d (%+v): no state in %d..%d (%d edits had returned, the reader was at %d); stale or unknown", i, o, lo, hi, o.lo, floor)
		}
		floor = j
	}
	return nil
}

// place is the earliest state in lo..hi an observation fits, or -1.
func (l *stressLog) place(index map[canon.Revision]int, o observation, lo, hi int) int {
	for j := lo; j <= hi; j++ {
		if l.fits(index, o, j) {
			return j
		}
	}
	return -1
}

// fits is whether state j has the observation's revision, if it has one, and every label it read.
func (l *stressLog) fits(index map[canon.Revision]int, o observation, j int) bool {
	if k, ok := index[o.rev]; o.rev != "" && (!ok || k != j) {
		return false
	}
	for i, label := range o.labels {
		if label != "" && l.states[j].labels[i] != label {
			return false
		}
	}
	return true
}

// stressReader loops over reads of the project, keeping one Value it compares at every turn.
type stressReader struct {
	p        *canon.Project
	log      *stressLog
	clock    *stressClock
	rng      *rand.Rand
	held     *canon.Value
	heldText string
	obs      []observation
}

// stressReads are the reads a reader draws among; each reports what it read.
var stressReads = []func(context.Context, *stressReader) (observation, error){
	readValue, readEvaluate, readCheck, readRevision,
}

func (r *stressReader) run(ctx context.Context) error {
	after := false
	for r.clock.more(ctx) {
		if r.held != nil && r.held.Text != r.heldText {
			return fmt.Errorf("a held Value changed: %s, was %s (API.md R4)", r.held.Text, r.heldText)
		}
		lo := int(r.log.committed.Load())
		o, err := stressReads[r.rng.IntN(len(stressReads))](ctx, r)
		if err != nil {
			return err
		}
		o.lo, o.hi = lo, int(r.log.committed.Load())+1
		r.obs = append(r.obs, o)
		if !after && lo >= 1 {
			after = true
			r.clock.readers.Add(1)
		}
	}
	return nil
}

// readValue reads the whole tiers value: its three labels come from one snapshot.
func readValue(ctx context.Context, r *stressReader) (observation, error) {
	v, err := r.p.Value(ctx, tiersPath)
	if err != nil {
		return observation{}, fmt.Errorf("Value: %w", err)
	}
	labels, err := tierLabels(v)
	if err != nil {
		return observation{}, err
	}
	if r.held == nil || r.rng.IntN(stressHeldOne) == 0 {
		r.held, r.heldText = v, v.Text
	}
	return observation{labels: labels}, nil
}

// readEvaluate reads an entry's title, its label (API.md V7), with the revision it was read at.
func readEvaluate(ctx context.Context, r *stressReader) (observation, error) {
	tier := r.rng.IntN(len(stressTiers))
	res, err := r.p.Evaluate(ctx, canon.EvalRequest{Path: entryPath(tier)})
	if err != nil {
		return observation{}, fmt.Errorf("Evaluate: %w", err)
	}
	if res.Path != entryPath(tier) || len(res.Findings) > 0 || !res.Title.OK || res.Title.Value == "" {
		return observation{}, fmt.Errorf("Evaluate %s: path %s, title %+v, findings %+v", entryPath(tier), res.Path, res.Title, res.Findings)
	}
	labels := make([]string, len(stressTiers))
	labels[tier] = res.Title.Value
	return observation{rev: res.Revision, labels: labels}, nil
}

// readCheck reads the warning naming the three labels, with the revision it was read at.
func readCheck(ctx context.Context, r *stressReader) (observation, error) {
	res, err := r.p.Check(ctx, stressPkg)
	if err != nil || res.HasErrors() {
		return observation{}, fmt.Errorf("Check: %v, %+v", err, res)
	}
	for _, f := range res.Findings {
		if text, ok := strings.CutPrefix(f.Message, labelsPrefix); ok {
			if labels := strings.Split(text, labelsSep); len(labels) == len(stressTiers) {
				return observation{rev: res.Revision, labels: labels}, nil
			}
		}
	}
	return observation{}, fmt.Errorf("Check: no warning naming the labels in %+v", res.Findings)
}

func readRevision(_ context.Context, r *stressReader) (observation, error) {
	return observation{rev: r.p.Revision()}, nil
}

func reads(readers []*stressReader) int {
	n := 0
	for _, r := range readers {
		n += len(r.obs)
	}
	return n
}

// stressWatch is what the watcher received, and whether fn ever ran twice at once (W13).
type stressWatch struct {
	mu       sync.Mutex
	events   []canon.Event
	inflight atomic.Int32
	overlap  atomic.Bool
}

func (w *stressWatch) event(ev canon.Event) {
	if w.inflight.Add(1) != 1 {
		w.overlap.Store(true)
	}
	defer w.inflight.Add(-1)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, ev)
}

// wait returns the events once one reports rev, the last edit's, and no other came for a
// while after it; or what came within eventWait.
func (w *stressWatch) wait(rev canon.Revision) []canon.Event {
	deadline := time.Now().Add(eventWait)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		seen := slices.ContainsFunc(w.events, func(ev canon.Event) bool { return ev.Revision == rev })
		w.mu.Unlock()
		if seen {
			time.Sleep(eventNone) // an event after the last edit's would be one too many (W15)
			break
		}
		time.Sleep(time.Millisecond)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.events)
}

// checkEvents is API.md W13 and W15: one event per edit, caused by the edit, in revision order,
// the n edits all reported, and fn never called twice at once.
func checkEvents(index map[canon.Revision]int, events []canon.Event, n int, overlap bool) error {
	if overlap {
		return errors.New("fn ran concurrently with itself (API.md W13)")
	}
	for i, ev := range events {
		j, ok := index[ev.Revision]
		switch {
		case ev.Err != nil || ev.Summary.Errors > 0:
			return fmt.Errorf("event %d: %v, %d errors", i, ev.Err, ev.Summary.Errors)
		case ev.Cause != canon.CauseEdit:
			return fmt.Errorf("event %d: cause %s for %v, want edit (API.md W15)", i, ev.Cause, ev.Files)
		case !ok || j != i+1:
			return fmt.Errorf("event %d reports state %d (known %v), want %d: out of order, repeated or a gap (API.md W13, W15)", i, j, ok, i+1)
		}
	}
	if len(events) != n {
		return fmt.Errorf("%d events for %d edits (API.md W15)", len(events), n)
	}
	return nil
}
