package ir

// BranchTables are the tables the cpp emit's decoders read, detail::k<T><F>Branches (CODEGEN.md §5.6, DECISIONS 350): every class of the package, which a data or types emit decodes, then each class of another package its readers read; none in baked mode, whose literals write the branch the evaluator decided.
func (pl *CppNamePlan) BranchTables() []NamedBranches {
	if pl.baked() || pl.tabled {
		return pl.tables
	}
	pl.tabled = true
	pl.tables = namedBranches(append(pl.classes(), readClasses(pl.foreign)...), func(class any, f *Field) string {
		return cppConstPrefix + pl.anyClassName(class) + cppUpperCamel(f.Name) + cppBranchesSuffix
	})
	return pl.tables
}

// BranchesName is the cpp table of field f, "" for a field no table of the emit holds.
func (pl *CppNamePlan) BranchesName(f *Field) string { return branchesOf(pl.BranchTables(), f) }

// anyClassName is the class name of a record or case of this package or another, as its owner names it.
func (pl *CppNamePlan) anyClassName(class any) string {
	if c, ok := class.(*Case); ok {
		if v := pl.foreign.VariantOf(c); v != nil {
			return pl.CaseName(v, c)
		}
	}
	return pl.className(class)
}

// declareBranchTables declares each cpp branch table in detail (CODEGEN.md §3.5, §5.6).
func (pl *CppNamePlan) declareBranchTables(detail *nameScope) {
	for _, b := range pl.BranchTables() {
		pl.declareInner(detail, b.Name, pl.p.Name+qnameSep+b.Name, b.Field)
	}
}
