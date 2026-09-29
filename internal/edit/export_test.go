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

// What a Region of a Write holds after it: any text, one node's text, nothing, one new item.
const (
	RegionAny  = int(regionAny)
	RegionNode = int(regionNode)
	RegionGone = int(regionGone)
	RegionItem = int(regionItem)
)

// Region is a byte range of a Write's before, and what the write may put there.
type Region struct {
	Lo, Hi, Kind int
}

// Write is one write Apply made of a file: its bytes before and after, and the regions of
// before its changes write (API.md M6).
type Write struct {
	Before, After []byte
	Regions       []Region
}

// Writes are the writes the plan made of the file it reports at display, in order.
func Writes(p *Plan, display string) []Write {
	var out []Write
	for _, s := range p.writes[display] {
		w := Write{Before: s.before, After: s.after}
		for _, r := range s.regions {
			w.Regions = append(w.Regions, Region{Lo: r.lo, Hi: r.hi, Kind: int(r.kind)})
		}
		out = append(out, w)
	}
	return out
}
