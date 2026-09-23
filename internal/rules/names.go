package rules

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Names reports a check name used twice in one record, case or package (EVALUATION.md §8.3).
func (r *Runner) Names(pkg *check.Package) error {
	bag, err := r.bagOf(pkg.Path)
	if err != nil {
		return err
	}
	var top []*syntax.CheckDecl
	for _, obj := range pkg.Decls {
		if c, ok := obj.Decl().(*syntax.CheckDecl); ok && obj.Kind() == check.ObjCheck {
			top = append(top, c)
		}
		if obj.Kind() == check.ObjTypeName {
			r.typeNames(obj.Type(), bag)
		}
	}
	r.unique(top, pkg.Path, bag)
	return nil
}

// typeNames checks the names of a record's checks, or of each case's of a variant.
func (r *Runner) typeNames(t types.Type, bag *diag.Bag) {
	if t == nil {
		return
	}
	switch d := t.Base().(type) {
	case *types.RecordType:
		r.unique(d.Checks, d.Name, bag)
	case *types.VariantType:
		for _, c := range d.Cases {
			r.unique(c.Checks, d.Name+dot+c.Name, bag)
		}
	}
}

func (r *Runner) unique(checks []*syntax.CheckDecl, scope string, bag *diag.Bag) {
	seen := map[string]bool{}
	for _, c := range checks {
		name := checkName(c)
		if name == "" {
			continue
		}
		if seen[name] {
			if file := r.files[c]; file != nil {
				diag.E5003.At(file.Span(c.Name), name, scope).Report(bag)
			}
			continue
		}
		seen[name] = true
	}
}
