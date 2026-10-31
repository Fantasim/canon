package rules

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Tests reports E5005 (EVALUATION.md §10.1) and E5004 (EVALUATION.md §10.3, §10.5).
func (r *Runner) Tests(pkg *check.Package) error {
	bag, err := r.bagOf(pkg.Path)
	if err != nil {
		return err
	}
	seen, subj := map[string]bool{}, subjectOf(pkg)
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
			r.expectNames(d, file, subj, bag)
		}
	}
	return nil
}

// subject is the test's package: its package checks and the types of its top-level values (EVALUATION.md §10.3).
type subject struct {
	path   string
	checks []*syntax.CheckDecl
	values []types.Type
}

func subjectOf(pkg *check.Package) subject {
	s := subject{path: pkg.Path}
	for _, obj := range pkg.Decls {
		if c, ok := obj.Decl().(*syntax.CheckDecl); ok && obj.Kind() == check.ObjCheck {
			s.checks = append(s.checks, c)
		}
		if k := obj.Kind(); (k == check.ObjConst || k == check.ObjLet) && obj.Type() != nil {
			s.values = append(s.values, obj.Type())
		}
	}
	return s
}

// expectNames is E5004 (EVALUATION.md §10.3, §10.5).
func (r *Runner) expectNames(d *syntax.TestDecl, file *syntax.File, pkg subject, bag *diag.Bag) {
	syntax.Inspect(d.Body, func(n syntax.Node) bool {
		x, ok := n.(*syntax.ExpectStmt)
		if !ok || x.Outcome == nil {
			return true
		}
		id, ok := x.Message.(*syntax.Ident)
		if !ok || !eval.NamesCheck(x.Outcome.Name, id.Name) {
			return true
		}
		if x.Package != nil {
			r.packageName(file.Span(id), id.Name, pkg, bag)
		} else if t := r.infoType(x.X); t != nil {
			r.valueName(file.Span(id), id.Name, t, pkg, bag)
		}
		return true
	})
}

func (r *Runner) infoType(x syntax.Expr) types.Type {
	if r.info == nil {
		return nil
	}
	return r.info.Types[x]
}

func (r *Runner) packageName(at source.Span, name string, pkg subject, bag *diag.Bag) {
	if hasCheck(pkg.checks, name) || slices.ContainsFunc(pkg.values, func(t types.Type) bool { return r.reaches(t, name) }) {
		return
	}
	diag.E5004.AtPackage(at, name, pkg.path).Report(bag)
}

func (r *Runner) valueName(at source.Span, name string, t types.Type, pkg subject, bag *diag.Bag) {
	switch {
	case r.reaches(t, name):
	case hasCheck(pkg.checks, name):
		diag.E5004.AtPackageCheck(at, name, t).Report(bag)
	default:
		diag.E5004.AtValue(at, name, t).Report(bag)
	}
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
