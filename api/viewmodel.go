package canon

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/workspace"
)

// viewModel is pkg's model as `emit view` writes it: the analysis of pkg and its imports, the
// model built from it and written by gen/view, errors or not (API.md R9, VIEWMODEL.md J1, J4);
// identical concurrent calls share it (S8).
func (p *Project) viewModel(ctx context.Context, pkg string) (*ViewModel, error) {
	s, err := p.read(ctx)
	if err != nil {
		return nil, err
	}
	if !project.IsPackageName(pkg) { // API.md §5.4: a package name, not a selector
		return nil, fmt.Errorf(fmtUnknown, ErrUnknownPackage, pkg)
	}
	data, err := share(ctx, s, workspace.Key(workspace.OpViewModel, []string{pkg}), func(ctx context.Context) ([]byte, error) {
		return viewModelData(ctx, s, pkg)
	})
	if err != nil {
		return nil, err
	}
	rev, err := p.revision(ctx, s)
	if err != nil {
		return nil, err
	}
	return &ViewModel{Package: pkg, Revision: rev, data: slices.Clone(data)}, nil
}

// viewModelData is pkg's model written by gen/view on snapshot s.
func viewModelData(ctx context.Context, s *workspace.Snapshot, pkg string) ([]byte, error) {
	a, err := analyze(ctx, s, []string{pkg})
	if err != nil {
		return nil, err
	}
	m, err := a.ViewModel(ctx, pkg)
	switch {
	case errors.Is(err, build.ErrNotSelected): // a selector naming no single package (R1)
		return nil, fmt.Errorf(fmtUnknown, ErrUnknownPackage, pkg)
	case err != nil:
		return nil, apiError(err)
	}
	data, err := viewgen.Write(m)
	if err != nil {
		return nil, internalError(err)
	}
	return data, nil
}
