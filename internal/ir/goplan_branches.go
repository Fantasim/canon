package ir

import "slices"

// NamedBranches is a build-time branch table an emit's decoders read, named as its target names it: the field, and the fields of the class holding it, which its discriminant's path reads (CODEGEN.md §5.6, DECISIONS 350).
type NamedBranches struct {
	Name   string
	Field  *Field
	Fields []*Field
	Origin string // the field's qualified Canon name, what an E8005 on the table names (DECISIONS 429)
	Owner  any    // the class the field belongs to, whose @<target>(name:) renames the table (E8005 derived, DECISIONS 458)
}

// namedBranches are the tables of the fields of classes whose dependent value is decided through a ref, each field once, in class then field order; name names one from its class and field.
func namedBranches(classes []any, name func(class any, f *Field) string, origin func(class any) string) []NamedBranches {
	var out []NamedBranches
	for _, class := range classes {
		fields, _ := classBody(class)
		for _, f := range fields {
			if refDecided(fields, f) && !slices.ContainsFunc(out, func(b NamedBranches) bool { return b.Field == f }) {
				out = append(out, NamedBranches{Name: name(class, f), Field: f, Fields: fields, Origin: origin(class) + qnameSep + f.Name, Owner: class})
			}
		}
	}
	return out
}

// classQName is a record's or case's qualified Canon name, a case's through the variant variantOf gives.
func classQName(class any, variantOf func(*Case) *Variant) string {
	switch x := class.(type) {
	case *Record:
		return x.QName()
	case *Case:
		if v := variantOf(x); v != nil {
			return v.QName() + qnameSep + x.Name
		}
		return x.Name
	}
	return ""
}

// ownVariants maps each case of p's own variants to its variant.
func ownVariants(p *Package) map[*Case]*Variant {
	out := map[*Case]*Variant{}
	for _, t := range p.Types {
		if v, ok := t.(*Variant); ok {
			for _, c := range v.Cases {
				out[c] = v
			}
		}
	}
	return out
}

// branchesOf is the name of the table of f among tables, "" for none.
func branchesOf(tables []NamedBranches, f *Field) string {
	i := slices.IndexFunc(tables, func(b NamedBranches) bool { return b.Field == f })
	if i < 0 {
		return ""
	}
	return tables[i].Name
}

// readClasses are the classes of other packages use reads, each variant as its cases.
func readClasses(use *ForeignUse) []any {
	var out []any
	for _, c := range use.Read {
		out = append(out, classAndCases(c)...)
	}
	return out
}

// BranchTables are the tables the go emit's decoders read, branches<T><F> (CODEGEN.md §5.6, DECISIONS 350), in decoder order: each decoded class of the package, then each class of another package its readers read; none in baked mode, whose literals write the branch the evaluator decided.
func (pl *GoNamePlan) BranchTables() []NamedBranches {
	if pl.data == nil || pl.branched {
		return pl.branches
	}
	pl.branched = true
	classes := slices.DeleteFunc(pl.classes(), func(c any) bool { return !pl.Decoded(c) })
	pl.branches = namedBranches(append(classes, readClasses(pl.foreign)...), func(class any, f *Field) string {
		name, own := pl.goNameOf[class]
		if !own {
			name = pl.foreignClassName(class)
		}
		return branchesPrefix + name + goUpperCamel(f.Name)
	}, func(class any) string {
		if o, own := pl.originOf[class]; own {
			return o
		}
		return classQName(class, pl.foreign.VariantOf)
	})
	return pl.branches
}

// BranchesName is the go table of field f, "" for a field no table of the emit holds.
func (pl *GoNamePlan) BranchesName(f *Field) string { return branchesOf(pl.BranchTables(), f) }

// foreignClassName is the Go type name of another package's record or case, as its owner names it.
func (pl *GoNamePlan) foreignClassName(class any) string {
	switch x := class.(type) {
	case *Record:
		return pl.TypeName(x)
	case *Case:
		if v := pl.foreign.VariantOf(x); v != nil {
			return pl.CaseName(v, x)
		}
	}
	return ""
}

// declareBranchTables declares each go branch table in the package scope (CODEGEN.md §3.5, §5.6).
func (pl *GoNamePlan) declareBranchTables(top *nameScope) {
	for _, b := range pl.BranchTables() {
		pl.noteDeclared(top, b.Name, foreignNote{derived: true, owner: b.Owner}, func() { pl.declare(top, b.Name, b.Origin, b.Field) })
	}
}
