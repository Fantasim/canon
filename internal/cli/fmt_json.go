package cli

import (
	"cmp"
	"errors"
	"fmt"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/syntax"
)

// formatJSONSources normalizes the JSON files the parsed sources load, numbers as written (meta/decisions/log-2026-09-29.md "--json-sources vs evaluates nothing").
func (r *fmtRun) formatJSONSources() error {
	var found []load.File
	scratch := diag.NewBag(&r.set, "")
	for _, src := range r.sources {
		from := path.Dir(src.Path)
		syntax.Inspect(syntax.Parse(src, syntax.FileSource, scratch), func(n syntax.Node) bool {
			if e, ok := n.(*syntax.LoadExpr); ok {
				found = append(found, load.JSONFiles(r.fsys, r.layout, from, e, scratch)...)
			}
			return true
		})
	}
	slices.SortFunc(found, func(a, b load.File) int {
		return cmp.Or(cmp.Compare(a.Display, b.Display), cmp.Compare(a.Abs, b.Abs))
	})
	for _, f := range found {
		if err := r.inv.ctx.Err(); err != nil {
			return err
		}
		if err := r.formatJSON(f); err != nil {
			return err
		}
	}
	return nil
}

// formatJSON notes a JSON source's canonical layout; a file that does not parse is left as it is (FORMATTER.md §14.1, §16).
func (r *fmtRun) formatJSON(f load.File) error {
	abs, ok := r.visit(f.Abs)
	if !ok {
		return nil
	}
	data, err := r.read(f.Display, abs)
	if err != nil {
		return err
	}
	src, err := r.set.Add(f.Display, abs, data)
	if err != nil {
		return fmt.Errorf(fmtArgs, f.Display, errTooLarge)
	}
	bag := diag.NewBag(&r.set, "")
	root, err := jsonsrc.Parse(src, bag)
	if err == nil {
		r.record(f.Display, abs, data, jsonsrc.Format(root))
		return nil
	}
	return r.rejectJSON(bag, f.Display, data, err)
}

// rejectJSON keeps the finding of a JSON source that does not parse; only an error jsonsrc does not document is returned.
func (r *fmtRun) rejectJSON(bag *diag.Bag, display string, data []byte, err error) error {
	switch {
	case errors.Is(err, jsonsrc.ErrSyntax), errors.Is(err, jsonsrc.ErrDuplicateKey):
	case errors.Is(err, jsonsrc.ErrEncoding):
		load.ReportEncoding(bag, display, data, err)
	default:
		return fmt.Errorf(fmtWrap, err)
	}
	r.reject(bag)
	return nil
}
