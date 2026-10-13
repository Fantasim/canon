package build

import (
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

// spelling sets o.respell when o, an existing text file, is on disk under another spelling its package's canon.outputs lists, the only one its directory lists: one file, which takes its declared spelling (DECISIONS 357).
func (r *run) spelling(o *output, own *ownership) error {
	if o.Target != ir.TargetText || o.listing {
		return nil
	}
	dir, base := project.DirOf(o.Abs), path.Base(o.Abs)
	names, err := r.writes().listing(r.p.fs, dir)
	if err != nil {
		return displayError(path.Dir(o.Path), err)
	}
	if slices.Contains(names, base) {
		return nil
	}
	i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, base) })
	if i < 0 {
		return nil
	}
	listed, err := own.names(o.Package)
	if _, ok := listed[path.Join(path.Dir(o.Path), names[i])]; ok && err == nil {
		o.respell = project.Join(dir, names[i])
	}
	return err
}
