package workspace

import (
	"context"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/source"
)

// scoped is the analysis of an edit's scope and the scope (API.md E17a, DECISIONS 330): the
// packages its ops name, found from the parsed sources, with every package importing one for a
// Rename or a RenameName; their imports and the studio come with them, the rest parsed only.
func (s *Snapshot) scoped(ctx context.Context, ops []edit.Operation, base string) (*build.Analysis, []string, error) {
	st, err := s.static(ctx)
	if err != nil {
		return nil, nil, err
	}
	pkgs, importers := edit.Scope(st.Units, ops)
	if importers {
		pkgs = importing(st.Units, pkgs...)
	} else if all := s.wholeScope(pkgs, base); all != nil {
		return all, pkgs, nil
	}
	a, err := AnalyzeOnly(ctx, s, pkgs)
	return a, pkgs, err
}

// noOps is an edit without operations: it analyses no package and writes nothing; its base is
// judged as any edit's (API.md E17a, S4, S5).
func (s *Snapshot) noOps(ctx context.Context, base string) (*EditOutcome, error) {
	if err := s.staleFor(ctx, base, nil, nil); err != nil {
		return nil, err
	}
	checked := &build.Result{Findings: build.Findings{Files: &source.FileSet{}}}
	return &EditOutcome{Plan: &edit.Plan{}, Checked: checked, Before: s, After: s}, nil
}

// staleConsulted is a *StaleError when, since base, a directory listing the re-check's asset
// checks consulted for the packages it re-checked changed on s (API.md S5), which only the
// re-check knows for a package outside the scope; nil for none or base "" (S6).
func (s *Snapshot) staleConsulted(ctx context.Context, base string, o *EditOutcome) error {
	if base == "" || o.checked == nil || s.current(base) {
		return nil
	}
	if fresh, err := s.fresh(ctx, base); err != nil || fresh {
		return err
	}
	var reads []build.Read
	for _, pkg := range o.rechecked {
		reads = append(reads, o.checked.Consulted(pkg)...)
	}
	return s.Stale(base, reads)
}
