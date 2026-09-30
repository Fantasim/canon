package workspace

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// kept is what a snapshot keeps of what was computed on it, which never changes (S8): every
// package's analysis, the latest analysis of a selection and its key, the packages its sources
// declare, and the selection whose analysis gives every package's values (Covered).
type kept struct {
	all     *build.Analysis
	one     *build.Analysis
	oneKey  string
	units   []*project.Unit
	covered string
}

// Analyze is the analysis of the packages selectors name on s, shared by identical concurrent
// calls (S8) and, once it succeeded, kept for later ones: every package's, and the latest of a
// selection (NFR-02).
func Analyze(ctx context.Context, s *Snapshot, selectors []string) (*build.Analysis, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := Key(OpAnalyze, selectors)
	if a := s.keptAnalysis(key, len(selectors) == 0); a != nil {
		return a, nil
	}
	return Share(ctx, s, key, func(ctx context.Context) (*build.Analysis, error) {
		a, err := s.b.Analyze(ctx, selectors)
		if err == nil {
			s.keepAnalysis(key, len(selectors) == 0, a)
		}
		return a, err
	})
}

// Covered reports that the analysis of pkgs on s gives every package's values (log-2026-09-29
// M4 P14-r): an edit found it so (build.Covers) when it published s.
func Covered(s *Snapshot, pkgs []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kept.covered != "" && s.kept.covered == Key(OpAnalyze, pkgs)
}

// cover notes that the analysis of pkgs on s gives every package's values.
func (s *Snapshot) cover(pkgs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kept.covered = Key(OpAnalyze, pkgs)
}

// keptAnalysis is the analysis s keeps under key, nil for none.
func (s *Snapshot) keptAnalysis(key string, every bool) *build.Analysis {
	s.mu.Lock()
	defer s.mu.Unlock()
	if every {
		return s.kept.all
	}
	if s.kept.oneKey == key {
		return s.kept.one
	}
	return nil
}

// keepAnalysis keeps a, computed on s under key, in place of the selection kept before, and the
// packages its snapshot parsed.
func (s *Snapshot) keepAnalysis(key string, every bool, a *build.Analysis) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if every {
		s.kept.all = a
	} else {
		s.kept.one, s.kept.oneKey = a, key
	}
	s.kept.units = a.Units()
}

// unkeep drops a from what s keeps: a failure a view met sticks to it (build.Analysis.ViewErr).
func (s *Snapshot) unkeep(a *build.Analysis) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kept.all == a {
		s.kept.all = nil
	}
	if s.kept.one == a {
		s.kept.one, s.kept.oneKey = nil, ""
	}
}

// units are the packages of s's sources: an analysis's kept on s, else scanned and parsed, shared (S8).
func (s *Snapshot) units(ctx context.Context) ([]*project.Unit, error) {
	s.mu.Lock()
	units := s.kept.units
	s.mu.Unlock()
	if units != nil {
		return units, nil
	}
	got, err := Share(ctx, s, Key(OpPackages, nil), s.b.Packages)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kept.units = got.Units
	return got.Units, nil
}

// Touching is pkgs and every package importing one, directly or not, by s's import clauses, in
// name order, unknown names left out: what an edit of pkgs affects (API.md E17) and an evaluation
// of their values touches (V13).
func Touching(ctx context.Context, s *Snapshot, pkgs ...string) ([]string, error) {
	units, err := s.units(ctx)
	if err != nil {
		return nil, err
	}
	return importing(units, pkgs...), nil
}

// importing is pkgs and each unit importing one, directly or not, in name order (API.md E17).
func importing(units []*project.Unit, pkgs ...string) []string {
	in := map[string]bool{}
	for _, u := range units {
		if slices.Contains(pkgs, u.Name) {
			in[u.Name] = true
		}
	}
	for grown := true; grown; {
		grown = false
		for _, u := range units {
			if !in[u.Name] && slices.ContainsFunc(u.Imports, func(i string) bool { return in[i] }) {
				in[u.Name], grown = true, true
			}
		}
	}
	out := make([]string, 0, len(in))
	for _, u := range units {
		if in[u.Name] {
			out = append(out, u.Name)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
