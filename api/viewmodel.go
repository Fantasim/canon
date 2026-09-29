package canon

import (
	"context"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/build"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/project"
)

// viewModel is pkg's model as `emit view` writes it: the analysis of pkg and its imports, the
// model built from it and written by gen/view, errors or not (API.md R9, VIEWMODEL.md J1, J4).
func (p *Project) viewModel(ctx context.Context, pkg string) (*ViewModel, error) {
	b, err := p.open()
	if err != nil {
		return nil, err
	}
	if !project.IsPackageName(pkg) { // API.md §5.4: a package name, not a selector
		return nil, fmt.Errorf(fmtUnknown, ErrUnknownPackage, pkg)
	}
	a, err := b.Analyze(ctx, []string{pkg})
	if err != nil {
		return nil, apiError(err)
	}
	p.setRevision(a.Result().Revision)
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
	return &ViewModel{Package: pkg, Revision: Revision(a.Result().Revision), data: data}, nil
}
