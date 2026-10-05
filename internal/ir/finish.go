package ir

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// finish fills what needs the whole IR: each value's `$schema`, the imported packages and the
// define tables the package's refs target.
func (s *stage) finish(u *unit) {
	first := firstJSONValue(u)
	for _, vs := range u.values {
		var fns []*ExportFn
		if vs.v.Name == first {
			fns = u.p.Fns
		}
		// Schema fails on a define record, which canon-fp has no form for (decision 126): validate refuses it where an emit needs the fingerprint (E8151, E8012, decision 194); check's phase 2 refuses its other failures.
		if id, err := Schema(u.p.Name, vs.v.Name, &vs.v.Type, fns); err == nil {
			vs.v.Schema = id
		}
	}
	u.p.Imports = s.imports(u)
	s.importSites(u)
	u.p.Defines = s.defines(u)
}

// firstJSONValue is the value whose file carries `$fns`: the first of the json emit's values (WIRE.md §5.11, decisions 118 and 128); "" without a json emit or with `values: []`.
func firstJSONValue(u *unit) string {
	for _, es := range u.emits {
		if es.e.Target != TargetJSON {
			continue
		}
		names := selectedNames(u, es.e)
		if len(names) > 0 {
			return names[0]
		}
	}
	return ""
}

// selectedNames are the values an emit selects: its `values`, else every public value.
func selectedNames(u *unit, e *Emit) []string {
	if e.Values != nil {
		return e.Values
	}
	out := make([]string, len(u.values))
	for i, v := range u.values {
		out[i] = v.v.Name
	}
	return out
}

// imports are the packages whose types or collections the IR names directly, by name (EMT-06, CODEGEN.md §2.8): another package's type is referenced, never entered; then those only its code emits reach (reachImports).
func (s *stage) imports(u *unit) []*PackageRef {
	u.firstUse = map[string]string{}
	use := func(pkg, name string) {
		if _, seen := u.firstUse[pkg]; !seen && pkg != u.p.Name {
			u.firstUse[pkg] = name
		}
	}
	w := newWalker(func(n Type) bool {
		use(pkgOf(n), n.QName())
		return pkgOf(n) == u.p.Name
	}, func(t *TypeRef) {
		if t.Ref != nil && t.Ref.Coll == types.CollLet {
			use(t.Ref.Pkg, t.Ref.Pkg+qnameSep+t.Ref.Value)
		}
	})
	w.pkgRefs(u.p)
	var out []*PackageRef
	for _, name := range slices.Sorted(maps.Keys(u.firstUse)) {
		if dep := s.units[name]; dep != nil {
			out = append(out, &PackageRef{Name: name, Dir: dep.p.Dir, Emits: dep.p.Emits})
		}
	}
	out = append(out, s.reachImports(u)...)
	slices.SortFunc(out, func(a, b *PackageRef) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// pkgOf is the package that declares a named IR type.
func pkgOf(n Type) string {
	switch x := n.(type) {
	case *Record:
		return x.Pkg
	case *Enum:
		return x.Pkg
	case *Variant:
		return x.Pkg
	case *Dependent:
		return x.Pkg
	}
	return ""
}

// defineKey identifies one load.defines table by its package and value name.
type defineKey struct{ pkg, value string }

// defines are the load.defines tables the package's refs target, sorted by package then name, each with its entries sorted by name (CODEGEN.md §5.8).
func (s *stage) defines(u *unit) []*DefineTable {
	seen := map[defineKey]bool{}
	var out []*DefineTable
	w := newWalker(nil, func(t *TypeRef) {
		if t.Ref == nil || t.Ref.Coll != types.CollDefines {
			return
		}
		key := defineKey{t.Ref.Pkg, t.Ref.Value}
		if seen[key] {
			return
		}
		seen[key] = true
		if d := s.defineTable(t.Ref.Pkg, t.Ref.Value); d != nil {
			out = append(out, d)
		}
	})
	w.pkgRefs(u.p)
	slices.SortFunc(out, func(a, b *DefineTable) int {
		return cmp.Or(cmp.Compare(a.Pkg, b.Pkg), cmp.Compare(a.Value, b.Value))
	})
	return out
}

// defineTable is one define table's names and values, sorted by name.
func (s *stage) defineTable(pkg, name string) *DefineTable {
	v, ok := s.in.Host.Value(s.ctx, pkg, name)
	tbl, isTable := v.(*value.Table)
	if !ok || !isTable {
		return nil
	}
	entries := slices.Clone(tbl.Entries)
	slices.SortFunc(entries, func(a, b *value.Record) int { return cmp.Compare(a.Ident.Key.S, b.Ident.Key.S) })
	d := &DefineTable{Pkg: pkg, Value: name}
	for _, e := range entries {
		if len(e.Fields) == 0 {
			return nil
		}
		n, isInt := e.Fields[0].(*value.Int)
		if !isInt {
			return nil
		}
		d.Names = append(d.Names, e.Ident.Key.S)
		d.Values = append(d.Values, n.V)
	}
	return d
}

// tableIDs are a table value's keys in entry order, retired ones included (EMT-04).
func tableIDs(v value.Value) []string {
	tbl, ok := v.(*value.Table)
	if !ok {
		return nil
	}
	out := make([]string, len(tbl.Entries))
	for i, e := range tbl.Entries {
		out[i] = e.Ident.Key.Text()
	}
	return out
}

// importSites locates each import of the package at its first import declaration, in file path order: a name only an imported package gives is reported there (log-2026-10-06 "U1 review" 4); a package reached only through others has none.
func (s *stage) importSites(u *unit) {
	for _, f := range sourceFiles(u.cp) {
		for _, imp := range f.Imports {
			i := slices.IndexFunc(u.p.Imports, func(r *PackageRef) bool { return r.Name == importPath(imp) })
			if i < 0 {
				continue
			}
			if _, seen := s.nodeSites[u.p.Imports[i]]; !seen {
				s.nodeSites[u.p.Imports[i]] = declSite{file: f, node: imp}
			}
		}
	}
}

// importPath is an import declaration's package path.
func importPath(imp *syntax.Import) string {
	if imp.Path == nil {
		return ""
	}
	parts := make([]string, len(imp.Path.Parts))
	for i, p := range imp.Path.Parts {
		parts[i] = p.Name
	}
	return strings.Join(parts, qnameSep)
}
