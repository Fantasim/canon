package convert

import (
	"fmt"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// AdoptOut is the out of the emit json once --adopt takes over legacy, every path written in package directory dir (CLI.md §3.10 step 4, DECISIONS 269).
func AdoptOut(p *project.Project, dir string, out []string, list bool, legacy string) ([]string, error) {
	if !list {
		return []string{legacy}, nil
	}
	l, _ := project.NewLayout(p, layoutAnchor, nil, nil)
	adopted, ok := resolve(l, dir, legacy)
	if !ok {
		return nil, fmt.Errorf(fmtAdoptPath, ErrAdoptPath, legacy)
	}
	entries := make([]project.Path, len(out))
	for i, o := range out {
		if entries[i], ok = resolve(l, dir, o); !ok {
			return nil, fmt.Errorf(fmtAdoptPath, ErrAdoptPath, o)
		}
		if entries[i].Abs == adopted.Abs {
			return out, nil
		}
	}
	if slices.ContainsFunc(out, func(o string) bool { return !check.JSONFile(o) }) {
		return nil, ErrAdoptForm
	}
	owner := check.OwningRoot(p, path.Dir(adopted.Abs))
	for _, e := range entries {
		if check.OwningRoot(p, path.Dir(e.Abs)) == owner {
			return nil, fmt.Errorf(fmtAdoptRoot, ErrAdoptRoot, e.Display, adopted.Display, check.RootLabel(p, owner))
		}
	}
	return append(slices.Clone(out), legacy), nil
}

// resolve places a path written in the package directory dir (WIRE.md §2.2).
func resolve(l *project.Layout, dir, written string) (project.Path, bool) {
	return l.Resolve(written, dir, source.Span{}, diag.NewBag(nil, ""))
}
