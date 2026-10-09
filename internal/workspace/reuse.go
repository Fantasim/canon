package workspace

import (
	"slices"

	"github.com/fantasim/canonlang/internal/build"
)

// KeptSelection is the AnalyzeOnly of exactly pkgs an edit's re-check kept on s (API.md E18),
// which Analyze(pkgs) equals without an active layer (E1901); nil otherwise.
func KeptSelection(s *Snapshot, pkgs []string) *build.Analysis {
	if s.b.Layered() {
		return nil
	}
	names := slices.Sorted(slices.Values(pkgs))
	a := s.keptAnalysis(Key(opOnly, names), false)
	if a == nil || !slices.Equal(a.Selected(), names) {
		return nil
	}
	return a
}

// wholeScope is s's kept every-package analysis standing for an edit's scope pkgs, importers
// aside (API.md E17a): its ops read only values there, the same without an active layer (E1901)
// or any error (E15's refusals read all; a spent budget is E4401), base not judged (S5).
func (s *Snapshot) wholeScope(pkgs []string, base string) *build.Analysis {
	if len(pkgs) == 0 || s.b.Layered() || s.keptAnalysis(Key(opOnly, pkgs), false) != nil || base != "" && !s.current(base) {
		return nil
	}
	all := s.keptAnalysis(Key(OpAnalyze, nil), true)
	if all == nil || all.Result().Summary.Errors > 0 {
		return nil
	}
	return all
}
