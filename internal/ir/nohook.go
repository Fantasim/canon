package ir

import "slices"

// noHooks fills NoHook of every data-mode go and cpp emit, each copy, of every package a selected package imports, directly or not: the records and cases (V.Case) its own plan writes no make hook for (CODEGEN.md §5.14, DECISIONS 363), so an importer reads the owner's decision instead of guessing it. Before the packages finish: an importer's decoded @text results read it (textImports). Only an importer reads it, and a type of another package is reached only through imports, so the transitive imports of the selected packages suffice.
func (s *stage) noHooks() {
	imported := s.importedClosure()
	for _, u := range s.order {
		if !imported[u.p.Name] {
			continue
		}
		for _, es := range u.emits {
			if es.e.Mode != ModeData || es.e.Target != TargetGo && es.e.Target != TargetCpp {
				continue
			}
			if written := s.ownerWrites(ownerKey{u.p.Name, es.e.Target}); written != nil {
				es.e.NoHook = unwritten(s.ownerIR(u), written)
			}
		}
	}
}

// importedClosure are the packages the selected ones import, directly or through each other's imports.
func (s *stage) importedClosure() map[string]bool {
	seen := map[string]bool{}
	var visit func(u *unit)
	visit = func(u *unit) {
		for _, f := range sourceFiles(u.cp) {
			for _, imp := range f.Imports {
				if dep := s.units[importPath(imp)]; dep != nil && !seen[dep.p.Name] {
					seen[dep.p.Name] = true
					visit(dep)
				}
			}
		}
	}
	for _, u := range s.order {
		if u.selected {
			visit(u)
		}
	}
	return seen
}

// unwritten are p's records and cases, as NoHook names them (Name, Variant.Case), whose make hook written says is not written, in declaration order.
func unwritten(p *Package, written func(Type, *Case) bool) []string {
	var out []string
	for _, t := range p.Types {
		out = append(out, unwrittenOf(t, written)...)
	}
	return slices.Clip(out)
}

// unwrittenOf is a record's name, or each case's Variant.Case, whose hook written says is not written; nothing for another type.
func unwrittenOf(t Type, written func(Type, *Case) bool) []string {
	var out []string
	switch x := t.(type) {
	case *Record:
		if !written(x, nil) {
			out = append(out, x.Name)
		}
	case *Variant:
		for _, c := range x.Cases {
			if !written(x, c) {
				out = append(out, x.Name+qnameSep+c.Name)
			}
		}
	}
	return out
}
