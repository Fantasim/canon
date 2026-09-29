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
	recs []record
	seq  int
	tip  map[name]sum // the newest record, whole
	fs   *snapFS      // the snapshot, and its entry count, the newest record was taken from
	gen  int
}

// add remembers rev with the entries of fs; the same revision again only gains fs's new entries.
func (h *history) add(rev string, fs *snapFS) {
	sums, gen := fs.sums()
	if n := len(h.recs); n > 0 && h.recs[n-1].rev == rev {
		if h.fs == fs && h.gen == gen {
			return
		}
		h.merge(sums)
	} else {
		h.push(rev, sums)
	}
	h.fs, h.gen = fs, gen
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

// push appends a record: whole every fullEvery, else the delta from the one before; past
// historyLen the oldest is dropped, the next made whole when it was a delta.
func (h *history) push(rev string, sums map[name]sum) {
	h.seq++
	r := record{rev: rev, seq: h.seq, full: h.seq%fullEvery == 0 || len(h.recs) == 0, sums: sums}
	if !r.full {
		r.sums = delta(h.tip, sums)
	}
	h.tip = maps.Clone(sums)
	h.recs = append(h.recs, r)
	if len(h.recs) > historyLen {
		if !h.recs[1].full {
			h.recs[1] = record{rev: h.recs[1].rev, seq: h.recs[1].seq, full: true, sums: h.whole(1)}
		}
		h.recs = h.recs[1:]
	}
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
