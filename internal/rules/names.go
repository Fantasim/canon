package rules

import (
	"slices"

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

// typeNames checks the names of a record's checks, or of each case's of a variant, which shares the variant-level checks' names (EVALUATION.md §8.3).
func (r *Runner) typeNames(t types.Type, bag *diag.Bag) {
	if t == nil {
		return
	}
	switch d := t.Base().(type) {
	case *types.RecordType:
		r.unique(d.Checks, d.Name, bag)
	case *types.VariantType:
		shared := r.shared[d]
		r.unique(shared, d.Name, bag)
		for _, c := range d.Cases {
			r.unique(c.Checks, d.Name+dot+c.Name, bag)
			r.sharedNames(shared, c.Checks, d.Name+dot+c.Name, bag)
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

// sharedNames is E5003 for a case's check named like a variant-level check, at the later of the two in source (EVALUATION.md §8.3).
func (r *Runner) sharedNames(shared, own []*syntax.CheckDecl, scope string, bag *diag.Bag) {
	for _, c := range own {
		name := checkName(c)
		i := slices.IndexFunc(shared, func(v *syntax.CheckDecl) bool { return name != "" && checkName(v) == name })
		if i < 0 {
			continue
		}
		later := c
		if shared[i].First() > c.First() {
			later = shared[i]
		}
		if file := r.files[later]; file != nil {
			diag.E5003.At(file.Span(later.Name), name, scope).Report(bag)
		}
	}
}
