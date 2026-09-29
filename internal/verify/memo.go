package verify

import (
	"strings"

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
	scoped  map[string]*scopedFound
}

// scopedFound is what the judgements reading the scope found in one scope, at paths under prefix.
type scopedFound struct {
	prefix string
	found  []reported
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
	unbound int
}

// memoScope is what the scope's judgements read: the enclosing entry E3502 names, and whether a retired one encloses (LOCK.md §4.3).
func memoScope(sc scope) string {
	if sc.retired {
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

// flagScoped is flag for a judgement that reads the scope (E3506, E3502): kept per scope.
func (w *walker) flagScoped(s Site, b *diag.Builder, v value.Value, at *Path) {
	s.Report(b, at, w.bag)
	w.noteFound(b)
	w.invalid(v)
	for _, r := range w.recording {
		r.scoped = append(r.scoped, reported{b: b, path: at.String()})
	}
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
	m := &memo{rec: cp, invalid: invalid && len(rec.found) > 0, bag: w.bag, prefix: here, found: rec.found}
	m.scoped = map[string]*scopedFound{memoScope(sc): {prefix: here, found: rec.scoped}}
	m.unbound = append(m.unbound, w.res.Unbound[rec.unbound:]...)
	w.stage.Remember(r, cp, m)
	return nv
}

// replay is a remembered instance's copy, judged in a new scope, its findings again in another bag (EVALUATION.md §10.2).
func (w *walker) replay(m *memo, t types.Type, at *Path, sc scope) value.Value {
	w.res.Valid = w.res.Valid && !m.invalid
	here, key := at.String(), memoScope(sc)
	s, judged := m.scoped[key]
	if !judged {
		rec := &recorder{}
		w.recording = append(w.recording, rec)
		w.rejudge(m.rec, t, at, sc)
		w.recording = w.recording[:len(w.recording)-1]
		m.scoped[key] = &scopedFound{prefix: here, found: rec.scoped}
	}
	if m.bag == w.bag {
		return m.rec
	}
	w.replayed(m.found, m.prefix, here)
	if judged {
		w.res.Valid = w.res.Valid && len(s.found) == 0
		w.replayed(s.found, s.prefix, here)
	}
	for _, u := range m.unbound {
		w.res.Unbound = append(w.res.Unbound, Unbound{Ref: u.Ref, Path: here + strings.TrimPrefix(u.Path, m.prefix)})
	}
	return m.rec
}

// replayed reports found again into this walk's bag, a path under prefix rebased under here.
func (w *walker) replayed(found []reported, prefix, here string) {
	for _, f := range found {
		path := f.path
		if path != "" {
			path = here + strings.TrimPrefix(f.path, prefix)
			f.b.Path(path)
		}
		f.b.Report(w.bag)
		w.keep(f.b, path)
	}
}
