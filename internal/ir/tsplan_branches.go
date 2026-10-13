package ir

// TSBranchTables are the tables a ts emit's readers may read, branches<T><F>, T the class's TypeScript name (CODEGEN.md §5.6, DECISIONS 350), in data and types mode: each record and case of the package, then each of another package the file reaches; gen/ts writes those its readers read, and the plan declares them all, as it declares the readers.
func TSBranchTables(p *Package, e *Emit) []NamedBranches {
	if e.Mode != ModeData && e.Mode != ModeTypes {
		return nil
	}
	var classes []any
	names := map[any]string{}
	add := func(t Type) {
		switch x := t.(type) {
		case *Record:
			classes, names[x] = append(classes, x), tsName(x)
		case *Variant:
			for _, c := range x.Cases {
				classes, names[c] = append(classes, c), tsCaseName(x, c)
			}
		}
	}
	for _, t := range p.Types {
		add(t)
	}
	seen := map[Type]bool{}
	tsReached(p, func(t *TypeRef) {
		if t.Named != nil && pkgOf(t.Named) != p.Name && !seen[t.Named] {
			seen[t.Named] = true
			add(t.Named)
		}
	})
	return namedBranches(classes, func(class any, f *Field) string {
		return branchesPrefix + names[class] + cppUpperCamel(f.Name)
	})
}

// declareBranchTables declares each ts branch table in the module scope (CODEGEN.md §3.5, §5.6).
func (pl *tsNamePlan) declareBranchTables(mod *nameScope) {
	for _, b := range TSBranchTables(pl.p, pl.e) {
		pl.declare(mod, b.Name, pl.origin(b.Name), b.Field)
	}
}
