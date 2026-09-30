package edit

import (
	"context"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// RefHistory is how many values Refs visits looking for r's target in s, and how many values
// it hands History, summed over its calls (log-2026-09-29 M4 U4a: the History cost).
func RefHistory(ctx context.Context, s *Snapshot, r Resolved) (visits, handed int, err error) {
	res, err := s.reopen(r)
	if err != nil {
		return 0, 0, err
	}
	tg, ok := s.target(res)
	if !ok {
		return 0, 0, ErrBadOp
	}
	sc := s.newScan(tg)
	inner := sc.history
	sc.history = func(path ...value.Value) []value.Value {
		handed += len(path)
		return inner(path...)
	}
	err = sc.lets(ctx)
	return sc.visits, handed, err
}

// Covered is whether a name at sp lies inside one of covers, sorted as Refs sorts them.
func Covered(covers []source.Span, sp source.Span) bool {
	sc := &nameScan{covers: covers, reach: reaches(covers)}
	return sc.covered(sp)
}

// RefVisits is how many values Refs visits looking for r's target in s.
func RefVisits(ctx context.Context, s *Snapshot, r Resolved) (int, error) {
	res, err := s.reopen(r)
	if err != nil {
		return 0, err
	}
	tg, ok := s.target(res)
	if !ok {
		return 0, ErrBadOp
	}
	sc := s.newScan(tg)
	err = sc.lets(ctx)
	return sc.visits, err
}

// The package's tests keep every write's steps (API.md M6); production Apply does not.
func init() { recordWrites = true }

// What a Region of a Write holds after it: any text, one node's text, nothing, one new item,
// a moved item's own lines, a comma or none.
const (
	RegionAny   = int(regionAny)
	RegionNode  = int(regionNode)
	RegionGone  = int(regionGone)
	RegionItem  = int(regionItem)
	RegionMoved = int(regionMoved)
	RegionComma = int(regionComma)
)

// Region is a byte range of a Write's before, and what the write may put there; a RegionMoved
// region carries the moved lines FromLo to FromHi, whose comma may change at Comma.
type Region struct {
	Lo, Hi, Kind          int
	FromLo, FromHi, Comma int
}

// Write is one write Apply made of a file: its bytes before and after, and the regions of
// before its changes write (API.md M6).
type Write struct {
	Before, After []byte
	Regions       []Region
	Op            int  // the index of the operation that made it, -1 for a cascade
	Reprint       bool // it only printed existing nodes again (API.md M1, M2)
}

// Writes are the writes the plan made of the file it reports at display, in order.
func Writes(p *Plan, display string) []Write {
	var out []Write
	for _, s := range p.writes[display] {
		w := Write{Before: s.before, After: s.after, Op: s.op, Reprint: s.reprint}
		for _, r := range s.regions {
			w.Regions = append(w.Regions, Region{Lo: r.lo, Hi: r.hi, Kind: int(r.kind), FromLo: r.fromLo, FromHi: r.fromHi, Comma: r.comma})
		}
		out = append(out, w)
	}
	return out
}

// StarKeys are the keys starNode writes map m's items under, one `*` level with nothing after
// it (WIRE.md 6.3, 5.8).
func StarKeys(m *value.Map) ([]string, error) {
	d := &jsonDiff{a: &applier{}}
	n, err := d.starNode(m, nil, nil)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, mb := range n.Members {
		out = append(out, mb.Key)
	}
	return out, nil
}

// CanonicalJSON is raw, the JSON source at display, in the canonical form M9 judges it against
// in s (API.md M9: layout and typed numbers).
func CanonicalJSON(s *Snapshot, display string, raw []byte) ([]byte, error) {
	a := &applier{snap: s}
	return a.canonical(display, raw, true)
}

// JSONSources are the JSON sources the roots of s read, display path to the content read.
func JSONSources(s *Snapshot) map[string][]byte {
	out := map[string][]byte{}
	var scan func(v value.Value)
	scan = func(v value.Value) {
		if p := provOf(v); p != nil && p.Kind == value.ProvJSON {
			out[s.display(p.Span.File)] = s.a.Files().Content(p.Span.File)
			return
		}
		for _, c := range children(v) {
			scan(c.v)
		}
	}
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if v, err := s.force(rootRef{pkg: pkg, obj: obj}); err == nil && initializer(obj.Decl()) != nil {
				scan(v)
			}
		}
	}
	return out
}
