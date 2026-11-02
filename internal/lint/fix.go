package lint

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// fixable reports that renaming field of h to name is a fix canon edit accepts (API.md L10, E29, E32).
func (p *pass) fixable(h holder, field *types.Field, name string) bool {
	if !project.IsIdent(name) || field.Stable || p.text[h.typ] || !p.safe(h.pkg) {
		return false
	}
	for _, f := range h.fields {
		if f != field && f.Name == name {
			return false
		}
	}
	for _, m := range h.methods {
		if m.Name == name {
			return false
		}
	}
	return true
}

// renamePath is the E27 path of the field called name in h: `<pkg>:<Rec>.<field>`, a case's with its variant.
func renamePath(h holder, name string) string {
	return h.pkg + packageMark + h.path + pathDot + name
}

// textReach are the records and cases that the result of a @text fn of any loaded package reaches
// through fields, lists, maps, optionals and tables, never a ref (E34 keeps the old wire name for
// loads and data emits only, so renaming a field such a result writes would change an output).
func textReach(prog *check.Program) map[types.Type]bool {
	seen := map[types.Type]bool{}
	for _, pkg := range prog.Packages {
		for _, fn := range textFns(pkg) {
			reach(seen, resultOf(prog, fn))
		}
	}
	return seen
}

// textFns are the top-level fns of pkg annotated @text.
func textFns(pkg *check.Package) []*syntax.FnDecl {
	var out []*syntax.FnDecl
	for _, f := range sources(pkg) {
		for _, d := range f.Decls {
			if fn, ok := d.(*syntax.FnDecl); ok && isText(fn) {
				out = append(out, fn)
			}
		}
	}
	return out
}

// isText reports a fn annotated @text.
func isText(fn *syntax.FnDecl) bool {
	for _, a := range fn.Annotations {
		if a.Name != nil && a.Name.Name == syntax.AnnText {
			return true
		}
	}
	return false
}

// resultOf is the result type of fn, nil when the checker could not type it.
func resultOf(prog *check.Program, fn *syntax.FnDecl) types.Type {
	obj := prog.Info.Defs[fn.Name]
	if obj == nil {
		return nil
	}
	ft, ok := obj.Type().(*types.FuncType)
	if !ok {
		return nil
	}
	return ft.Result
}

// reach marks the records and cases t reaches.
func reach(seen map[types.Type]bool, t types.Type) {
	if t == nil {
		return
	}
	switch b := t.Base().(type) {
	case *types.RecordType:
		reachFields(seen, b, b.Fields)
	case *types.AppliedRecord:
		reachFields(seen, b.Rec, b.Rec.Fields)
	case *types.CaseType:
		reachFields(seen, b, b.Fields)
	case *types.VariantType:
		for _, c := range b.Cases {
			reach(seen, c)
		}
	case *types.ListType:
		reach(seen, b.Elem)
	case *types.MapType:
		reach(seen, b.Value)
	case *types.DepMapType:
		reach(seen, b.Value)
	case *types.TypeAppType:
		reachBranches(seen, b, b.Fn)
	case *types.DepUnionType:
		reachBranches(seen, b, b.Fn)
	case *types.OptionalType:
		reach(seen, b.Elem)
	case *types.TableType:
		reach(seen, b.Elem)
	}
}

// reachFields marks key and what its fields reach, once.
func reachFields(seen map[types.Type]bool, key types.Type, fields []*types.Field) {
	if seen[key] {
		return
	}
	seen[key] = true
	for _, f := range fields {
		reach(seen, f.Type)
	}
}

// reachBranches marks what the branches of fn reach, once for the application key.
func reachBranches(seen map[types.Type]bool, key types.Type, fn *types.TypeFunc) {
	if seen[key] {
		return
	}
	seen[key] = true
	for _, t := range types.Branches(fn) {
		reach(seen, t)
	}
}
