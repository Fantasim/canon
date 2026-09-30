package workspace

import (
	"maps"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
)

// record is one revision the project produced and the content key of every entry its snapshot
// held: all of them in a full record, else those that differ from the record before, a name
// the snapshot no longer holds marked gone.
type record struct {
	rev  string
	seq  int
	full bool
	sums map[name]sum
}

// history is the last historyLen revisions a project produced, oldest first (API.md S4).
type history struct {
	recs  []record
	seq   int
	tip   map[name]sum // the newest record, whole
	fs    *snapFS      // the snapshot the newest record was taken from, when its log was logAt long
	logAt int
	own   bool // the newest record holds only entries fs holds: none merged from another snapshot
	exact bool // tip is what fs held when its log was logAt long, no more
}

// add remembers rev with the entries of fs; the same revision again only gains fs's new entries.
func (h *history) add(rev string, fs *snapFS) {
	if h.from(rev, fs) && h.logAt == fs.logLen() {
		return
	}
	d, at, ok := h.since(fs)
	n := len(h.recs)
	switch {
	case n > 0 && h.recs[n-1].rev == rev && ok:
		h.own = h.own && h.fs == fs
		h.mergeDelta(d)
	case n > 0 && h.recs[n-1].rev == rev:
		var sums map[name]sum
		sums, at = fs.sums()
		h.own = h.own && h.fs == fs
		h.merge(sums)
		h.exact = false
	case ok:
		h.push(rev, d)
	default:
		var sums map[name]sum
		sums, at = fs.sums()
		h.push(rev, delta(h.tip, sums))
	}
	h.fs, h.logAt = fs, at
}

// from reports that rev is the newest record and holds only what fs held when it was taken or
// read since: an entry fs holds never changes, so each one the record holds is fs's now, and a
// name fs reads later is one no record holds (at, API.md S5).
func (h *history) from(rev string, fs *snapFS) bool {
	n := len(h.recs)
	return n > 0 && h.recs[n-1].rev == rev && h.fs == fs && h.own
}

// merge adds the entries the newest record lacks or holds differently.
func (h *history) merge(sums map[name]sum) {
	last := &h.recs[len(h.recs)-1]
	//canon:unordered each entry is compared under its own name
	for n, v := range sums {
		if old, ok := h.tip[n]; !ok || old != v {
			last.sums[n] = v
			h.tip[n] = v
		}
	}
}

// mergeDelta is merge of the entries d, a delta from the tip, holds: a name gone is kept, so the
// tip is no longer exact.
func (h *history) mergeDelta(d map[name]sum) {
	last := &h.recs[len(h.recs)-1]
	//canon:unordered each entry is merged under its own name
	for n, v := range d {
		if v.class == classGone {
			h.exact = false
			continue
		}
		last.sums[n] = v
		h.tip[n] = v
	}
}

// push appends a record of d, the delta from the newest: whole every fullEvery, else d; past
// historyLen the oldest is dropped, the next made whole when it was a delta.
func (h *history) push(rev string, d map[name]sum) {
	h.seq++
	if h.tip == nil {
		h.tip = map[name]sum{}
	}
	//canon:unordered each entry is applied under its own name
	for n, v := range d {
		if v.class == classGone {
			delete(h.tip, n)
		} else {
			h.tip[n] = v
		}
	}
	h.exact, h.own = true, true
	r := record{rev: rev, seq: h.seq, full: h.seq%fullEvery == 0 || len(h.recs) == 0, sums: d}
	if r.full {
		r.sums = maps.Clone(h.tip)
	}
	h.recs = append(h.recs, r)
	if len(h.recs) > historyLen {
		if !h.recs[1].full {
			h.recs[1] = record{rev: h.recs[1].rev, seq: h.recs[1].seq, full: true, sums: h.whole(1)}
		}
		h.recs = h.recs[1:]
	}
}

// since is what differs in fs from the tip, told from the names stored or dropped since the tip
// was taken, and fs's log length; false when the tip is not exact or fs does not descend from the
// tip's snapshot by one fork at most (log-2026-09-29 M4 P18).
func (h *history) since(fs *snapFS) (map[name]sum, int, bool) {
	if !h.exact || h.fs == nil {
		return nil, 0, false
	}
	names, at, ok := fs.changedSince(h.fs, h.logAt)
	if !ok {
		return nil, 0, false
	}
	return fs.deltaOf(h.tip, names), at, true
}

// whole is record i's entries, rebuilt from the full record before it and the deltas since.
func (h *history) whole(i int) map[name]sum {
	j := i
	for !h.recs[j].full {
		j--
	}
	out := maps.Clone(h.recs[j].sums)
	for k := j + 1; k <= i; k++ {
		maps.Copy(out, h.recs[k].sums)
	}
	maps.DeleteFunc(out, func(_ name, v sum) bool { return v.class == classGone })
	return out
}

// delta is what differs in next from prev: a changed or new key, or gone for a name next lacks.
func delta(prev, next map[name]sum) map[name]sum {
	out := map[name]sum{}
	//canon:unordered each entry is compared under its own name
	for n, v := range next {
		if old, ok := prev[n]; !ok || old != v {
			out[n] = v
		}
	}
	//canon:unordered each entry is looked up under its own name
	for n := range prev {
		if _, ok := next[n]; !ok {
			out[n] = sum{class: classGone}
		}
	}
	return out
}

// find is the newest record of rev; false when it is not remembered (API.md S4).
func (h *history) find(rev string) (int, bool) {
	for i, r := range slices.Backward(h.recs) {
		if r.rev == rev {
			return i, true
		}
	}
	return 0, false
}

// at is n's content key at record i; a name record i does not hold has the key a later record
// first read it with; false when none did.
func (h *history) at(i int, n name) (sum, bool) {
	if v, ok := h.held(i, n); ok {
		return v, true
	}
	for k := i + 1; k < len(h.recs); k++ {
		if v, ok := h.recs[k].sums[n]; ok && v.class != classGone {
			return v, true
		}
	}
	return sum{}, false
}

// held is n's content key in record i itself, false when i does not hold n.
func (h *history) held(i int, n name) (sum, bool) {
	for k := i; k >= 0; k-- {
		if v, ok := h.recs[k].sums[n]; ok {
			return v, v.class != classGone
		}
		if h.recs[k].full {
			break
		}
	}
	return sum{}, false
}

// changed is the first of probes the history knows since record i, and whether it differs from
// it; one it never knew is unchanged.
func (h *history) changed(i int, probes []probe) (probe, bool) {
	for _, pr := range probes {
		if was, ok := h.at(i, pr.n); ok {
			return pr, was != pr.now
		}
	}
	return probe{}, false
}

// named is what a stale error names for r, whose probe pr changed since record i: each source
// of a package directory that record i never held, else r itself.
func (h *history) named(i int, r build.Read, pr probe) []string {
	var out []string
	for _, abs := range pr.sources {
		if _, ok := h.held(i, name{kind: kindFile, abs: abs}); !ok {
			out = append(out, path.Join(r.Display, path.Base(abs)))
		}
	}
	if len(out) == 0 {
		return []string{r.Display}
	}
	return out
}

// logLen is how many names s has stored or dropped so far.
func (s *snapFS) logLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.log)
}

// changedSince is every name whose entry in s may differ from base's when its log was at long:
// base's log between then and the fork, either first, and s's own; with s's log length, false
// when s is neither base nor forked from it.
func (s *snapFS) changedSince(base *snapFS, at int) ([]name, int, bool) {
	var names []name
	if s != base {
		if s.up.Value() != base {
			return nil, 0, false
		}
		base.mu.Lock()
		log := base.log
		base.mu.Unlock()
		names = slices.Clone(log[min(at, s.upAt):max(at, s.upAt)])
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s == base {
		return slices.Clone(s.log[at:]), len(s.log), true
	}
	return append(names, s.log...), len(s.log), true
}

// deltaOf is what differs in s from tip among names: a changed or new key, or gone for a name s
// lacks; a listing's key over its sources goes with the listing (sums).
func (s *snapFS) deltaOf(tip map[name]sum, names []name) map[name]sum {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[name]sum{}
	note := func(n name, v sum, ok bool) {
		old, had := tip[n]
		switch {
		case ok && (!had || old != v):
			out[n] = v
		case !ok && had:
			out[n] = sum{class: classGone}
		}
	}
	for _, n := range names {
		e, ok := s.ents[n]
		var v, src sum
		if ok {
			v, src = e.sum, e.src
		}
		note(n, v, ok)
		if n.kind == kindDir {
			note(name{kind: kindSources, abs: n.abs}, src, ok)
		}
	}
	return out
}
