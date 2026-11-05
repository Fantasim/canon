package workspace

import (
	"context"

	"github.com/fantasim/canonlang/internal/build"
)

// layered adds to o.Checked what each inactive layer of pkgs and their imports finds beyond the
// session, and to the staleness check what their analyses consulted (API.md E18, S5; DECISIONS 463).
func (o *EditOutcome) layered(ctx context.Context, checked *build.Analysis, pkgs []string, lang string) error {
	res, analyses, err := checked.Layered(ctx, o.Checked, build.LayerRun{Pkgs: pkgs, Lang: lang, Own: true})
	o.layers = analyses
	for _, a := range analyses {
		for _, pkg := range a.Selected() {
			o.layerReads = append(o.layerReads, a.Consulted(pkg)...)
		}
	}
	o.Checked = res
	return err
}

// keepEvent keeps on s, beside a, an edit's re-check, a's result with what layers, its inactive
// layers' analyses, add: what the edit's event reports, at no new analysis (API.md W13; DECISIONS 469).
func (s *Snapshot) keepEvent(ctx context.Context, a *build.Analysis, layers []*build.Analysis) error {
	res, err := a.Merged(ctx, a.Result(), layers, "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kept.event, s.kept.eventOf = res, a
	return nil
}

// Checked is a, an analysis of s, its result with what each inactive layer of its packages and their
// imports finds beyond it: an edit's kept on s (keepEvent), else computed and shared (API.md W13, S8;
// DECISIONS 469).
func Checked(ctx context.Context, s *Snapshot, a *build.Analysis) (*build.Result, error) {
	s.mu.Lock()
	res, of := s.kept.event, s.kept.eventOf
	s.mu.Unlock()
	if of == a && res != nil {
		return res, nil
	}
	// keyed by selection, not by a: one snapshot gives every analysis of a selection the same findings
	return Share(ctx, s, Key(opChecked, a.Selected()), func(ctx context.Context) (*build.Result, error) {
		out, _, err := a.Layered(ctx, a.Result(), build.LayerRun{Pkgs: a.Selected()})
		return out, err
	})
}

// Check is the check of the packages selectors name on s in lang ("" for Options.Lang): the analysis's
// result and what each inactive layer of those packages and their imports finds beyond it, shared by
// identical concurrent calls (API.md R2, R3a, F7, F11, S8; DECISIONS 469).
func Check(ctx context.Context, s *Snapshot, selectors []string, lang string) (*build.Result, error) {
	a, err := Analyze(ctx, s, selectors)
	if err != nil {
		return nil, err
	}
	res := a.Result()
	if lang != "" {
		in := *res
		if in.List, err = a.ResultIn(ctx, lang); err != nil {
			return nil, err
		}
		res = &in
	}
	return Share(ctx, s, Key(opCheck, selectors, lang), func(ctx context.Context) (*build.Result, error) {
		out, _, err := a.Layered(ctx, res, build.LayerRun{Pkgs: a.Selected(), Lang: lang})
		return out, err
	})
}
