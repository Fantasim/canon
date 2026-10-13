package cli

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// selectable lists what a selector can name under root: every package, then the orphan directories (CLI.md §2.2).
func selectable(ctx context.Context, fsys project.FS, root string) ([]project.Listing, error) {
	names, err := project.Scan(fsys, root)
	if err != nil {
		return nil, fmt.Errorf(fmtArgs, dotSep, readCause(err))
	}
	set := &source.FileSet{}
	bag := diag.NewBag(set, "")
	r := &project.Reader{FS: fsys, Dir: root, Set: set, BagOf: func(string) *diag.Bag { return bag }}
	units, err := r.Parse(ctx, names)
	if err != nil {
		return nil, fmt.Errorf(fmtArgs, dotSep, readCause(err))
	}
	return project.Listings(units, r.Orphans), nil
}
