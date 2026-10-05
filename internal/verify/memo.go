package verify

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// memo is an instance's one copy and its findings under prefix, the scope-reading ones per scope (EVALUATION.md §4.2).
type memo struct {
	rec     *value.Record
	invalid bool // it had a non-scoped finding: a finding outside E3506 and E3502
	bag     *diag.Bag
	prefix  string
	found   []reported
	unbound []Unbound
	uses    []retiredUse // judged again in each new scope, at paths under prefix
	scoped  map[string]*scopedFound
}

// scopedFound is what the judgements reading the scope found in one scope, at paths under prefix, reported into bag.
type scopedFound struct {
	prefix string
	found  []reported
	bag    *diag.Bag
}

// reported is a finding as reported: its builder and the path it was reported at.
type reported struct {
	b    *diag.Builder
	path string
}

// recorder collects the findings of an instance being verified, and where its unbound refs begin.
type recorder struct {
	found   []reported
	scoped  []reported
	uses    []retiredUse
	unbound int
}

// memoScope is what the scope's judgements read: the enclosing entry E3502 names, and whether a retired one encloses (LOCK.md §4.3).
func memoScope(sc scope) string {
	if sc.retired != "" {
		return retiredMark + sc.entry
	}
	return sc.entry
}

// report reports a finding at s and keeps it for each instance being verified around it.
func (w *walker) report(s Site, b *diag.Builder, at *Path) {
	s.Report(b, at, w.bag)
	w.noteFound(b)
	w.keep(b, at.String())
}

// noteFound keeps a finding the recorded entry reported, apart from the values it names.
func (w *walker) noteFound(b *diag.Builder) {
	if w.rec != nil {
		w.rec.found = append(w.rec.found, b.Detached())
	}
}

// keepSince keeps the findings the evaluator reported since its n-th, at path ("" for none).
func (w *walker) keepSince(n int, path string) {
	for _, b := range w.stage.Emitted()[n:] {
		w.keep(b, path)
	}
}

func (w *walker) keep(b *diag.Builder, path string) {
	for _, r := range w.recording {
		r.found = append(r.found, reported{b: b, path: path})
	}
}

// keepScoped keeps a finding of a judgement that reads the scope for each instance being verified around it.
func (w *walker) keepScoped(b *diag.Builder, path string) {
	for _, r := range w.recording {
		r.scoped = append(r.scoped, reported{b: b, path: path})
	}
}

// remembered verifies rec as walk does, then keeps what it made when it converted or charged
// something, so every other root or entry reaching the instance reuses its copy.
func (w *walker) remembered(r *value.Record, t types.Type, at *Path, sc scope) value.Value {
	valid, charged := w.res.Valid, w.charged
	rec := &recorder{unbound: len(w.res.Unbound)}
	w.recording = append(w.recording, rec)
	w.res.Valid = true
	nv := w.fields(r, t, at, sc)
	w.recording = w.recording[:len(w.recording)-1]
	invalid := !w.res.Valid
	w.res.Valid = valid && !invalid
	cp, ok := nv.(*value.Record)
	if !ok || w.stopped() || cp == r && w.charged == charged {
		return nv
	}
	here := at.String()
	m := &memo{rec: cp, invalid: invalid && len(rec.found) > 0, bag: w.bag, prefix: here, found: rec.found, uses: rec.uses}
	m.scoped = map[string]*scopedFound{memoScope(sc): {prefix: here, found: rec.scoped, bag: w.bag}}
	m.unbound = append(m.unbound, w.res.Unbound[rec.unbound:]...)
	w.stage.Remember(r, cp, m)
	return nv
}

// replay is a remembered instance's copy, its uses judged in a new scope and kept around it, its findings again in another bag (EVALUATION.md §10.2).
func (w *walker) replay(m *memo, at *Path, sc scope) value.Value {
	w.res.Valid = w.res.Valid && !m.invalid
	here := at.String()
	uses := rebased(m.uses, m.prefix, here, sc)
	for _, r := range w.recording {
		r.uses = append(r.uses, uses...)
	}
	if m.bag == w.bag {
		w.kept(m.found, m.prefix, here, w.keep)
	} else {
		w.replayed(m.found, m.prefix, here, w.keep)
		for _, u := range m.unbound {
			w.res.Unbound = append(w.res.Unbound, Unbound{Ref: u.Ref, Path: rebase(u.Path, m.prefix, here)})
		}
	}
	w.scopedIn(m, uses, here, sc)
	return m.rec
}

// scopedIn judges uses in sc once per scope, and reports again into this bag what a judgement in another bag found.
func (w *walker) scopedIn(m *memo, uses []retiredUse, here string, sc scope) {
	key := memoScope(sc)
	s, judged := m.scoped[key]
	switch {
	case !judged:
		rec := &recorder{}
		w.recording = append(w.recording, rec)
		for _, u := range uses {
			w.judgeUse(u)
		}
		w.recording = w.recording[:len(w.recording)-1]
		m.scoped[key] = &scopedFound{prefix: here, found: rec.scoped, bag: w.bag}
	case s.bag == w.bag:
		w.res.Valid = w.res.Valid && len(s.found) == 0
		w.kept(s.found, s.prefix, here, w.keepScoped)
	default:
		w.res.Valid = w.res.Valid && len(s.found) == 0
		w.replayed(s.found, s.prefix, here, w.keepScoped)
	}
}

// replayed reports found again into this walk's bag, a path under prefix rebased under here,
// and keeps each finding as keep does.
func (w *walker) replayed(found []reported, prefix, here string, keep func(*diag.Builder, string)) {
	for _, f := range found {
		path := rebase(f.path, prefix, here)
		if path != "" {
			f.b.Path(path)
		}
		f.b.Report(w.bag)
		keep(f.b, path)
	}
}

// kept keeps found, already in this walk's bag, for the instances being verified around it, a path under prefix rebased under here.
func (w *walker) kept(found []reported, prefix, here string, keep func(*diag.Builder, string)) {
	for _, f := range found {
		keep(f.b, rebase(f.path, prefix, here))
	}
}
