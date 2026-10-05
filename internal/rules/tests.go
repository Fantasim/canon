package rules

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Tests reports E5005 (EVALUATION.md §10.1) and E5004 (EVALUATION.md §10.3).
func (r *Runner) Tests(pkg *check.Package) error {
	bag, err := r.bagOf(pkg.Path)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, obj := range pkg.Decls {
		d, ok := obj.Decl().(*syntax.TestDecl)
		if !ok || obj.Kind() != check.ObjTest {
			continue
		}
		file := obj.File()
		if name := eval.TestName(d); name != "" && seen[name] {
			diag.E5005.At(file.Span(d.Name), name, pkg.Path).Report(bag)
		} else {
			seen[name] = true
		}
		if !r.isBroken(obj) {
			r.expectNames(d, file, bag)
		}
	}
	return nil
}

// expectNames is E5004: only checks of types reachable from the subject count (EVALUATION.md §10.2).
func (r *Runner) expectNames(d *syntax.TestDecl, file *syntax.File, bag *diag.Bag) {
	syntax.Inspect(d.Body, func(n syntax.Node) bool {
		x, ok := n.(*syntax.ExpectStmt)
		if !ok || x.Outcome == nil || r.info == nil {
			return true
		}
		id, ok := x.Message.(*syntax.Ident)
		if !ok || !eval.NamesCheck(x.Outcome.Name, id.Name) {
			return true
		}
		if t := r.info.Types[x.X]; t != nil && !r.reaches(t, id.Name) {
			diag.E5004.At(file.Span(id), id.Name, t).Report(bag)
		}
		return true
	})
}

// reaches reports a check called name on a type reachable from t, through fields, elements,
// cases and dependent branches.
func (r *Runner) reaches(t types.Type, name string) bool {
	seen := map[types.Type]bool{}
	var walk func(types.Type) bool
	walk = func(t types.Type) bool {
		t = t.Base()
		if seen[t] {
			return false
		}
		seen[t] = true
		shared, own := r.checksOf(t)
		if v, ok := t.(*types.VariantType); ok {
			shared = r.shared[v]
		}
		if hasCheck(shared, name) || hasCheck(own, name) {
			return true
		}
		return slices.ContainsFunc(components(t), walk)
	}
	return walk(t)
}

func hasCheck(cs []*syntax.CheckDecl, name string) bool {
	return slices.ContainsFunc(cs, func(c *syntax.CheckDecl) bool { return checkName(c) == name })
}

// components are the types a type holds directly: fields, elements, cases, dependent branches.
func components(t types.Type) []types.Type {
	switch d := t.(type) {
	case *types.RecordType:
		return fieldTypes(d.Fields)
	case *types.AppliedRecord:
		return fieldTypes(d.Rec.Fields)
	case *types.CaseType:
		return fieldTypes(d.Fields)
	case *types.VariantType:
		return slices.Collect(func(yield func(types.Type) bool) {
			for _, c := range d.Cases {
				if !yield(c) {
					return
				}
			}
		})
	case *types.TypeAppType:
		return types.Branches(d.Fn)
	case *types.DepUnionType:
		return types.Branches(d.Fn)
	case *types.OptionalType:
		return []types.Type{d.Elem}
	case *types.ListType:
		return []types.Type{d.Elem}
	case *types.TableType:
		return []types.Type{d.Elem}
	case *types.MapType:
		return []types.Type{d.Key, d.Value}
	case *types.DepMapType:
		return []types.Type{d.Value}
	}
	return nil
}

func fieldTypes(fs []*types.Field) []types.Type {
	out := make([]types.Type, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Type)
	}
	return out
}
