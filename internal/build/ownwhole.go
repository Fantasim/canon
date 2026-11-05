package build

import (
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
)

// ownWhole is Own's findings in the source language, no bag cut at its limit (API.md F7), for the
// inactive-layer merge of an edit's re-check (E18; DECISIONS 463). Read-only, safe on a published
// analysis: the bags its limit cut were read whole before Analyze returned (wholeBags).
func (a *Analysis) ownWhole() []diag.Finding {
	r := a.r
	out := slices.Clone(a.bagWhole(""))
	for _, u := range r.selected {
		out = append(out, a.bagWhole(u.Name)...)
	}
	for _, o := range r.orphans {
		out = append(out, a.bagWhole(o.Name)...)
	}
	return out
}

// resultWhole is Result's findings in the source language, no bag cut at its limit (API.md F7):
// Own's and the errors of the packages the analysis only imports (DECISIONS 196), for the
// inactive-layer merge of a check (DECISIONS 469).
func (a *Analysis) resultWhole() []diag.Finding {
	out := a.ownWhole()
	for _, u := range a.r.loaded {
		if slices.Contains(a.r.selected, u) {
			continue
		}
		for _, f := range a.bagWhole(u.Name) {
			if f.Severity == diag.Error {
				out = append(out, f)
			}
		}
	}
	return out
}

// bagWhole is the findings of pkg's bag ("" the project's own), uncut, read-only; none for a bag
// the run never made.
func (a *Analysis) bagWhole(pkg string) []diag.Finding {
	if whole, ok := a.wholes[pkg]; ok {
		return whole
	}
	if pkg == "" {
		return a.r.s.own.Findings()
	}
	if b := a.r.s.bags[pkg]; b != nil {
		return b.Findings()
	}
	return nil
}

// wholeBags is the findings of each bag of r that its limit cut, uncut, by package ("" the
// project's own): read once the run is done and before its analysis is published, so no later
// read lifts a cut a concurrent reader sees (API.md F7, S8).
func (r *run) wholeBags() map[string][]diag.Finding {
	out := map[string][]diag.Finding{}
	note := func(pkg string, b *diag.Bag, limit int) {
		if len(b.Summary().Truncated) > 0 {
			out[pkg] = uncut(b, limit)
		}
	}
	note("", r.s.own, diag.DefaultMaxFindings) // the project's own bag keeps diag's default (NewBag)
	for _, u := range r.loaded {
		note(u.Name, r.s.bag(u.Name), r.p.findingLimit())
	}
	for _, o := range r.orphans {
		note(o.Name, r.s.bag(o.Name), r.p.findingLimit())
	}
	return out
}

// uncut is b's findings with no cut, the limit lifted for this read only and put back at limit,
// as unreported does.
func uncut(b *diag.Bag, limit int) []diag.Finding {
	b.Truncate(math.MaxInt)
	defer b.Truncate(limit)
	return b.Findings()
}
