package ir

import "github.com/fantasim/canonlang/internal/types"

// TSOpenNames are the names an opened enum adds in TypeScript (CODEGEN.md §5.2, DECISIONS 362): `<E>Known`, the closed union; `is<E>Known`; `<e>Name`, e lowerCamel of the enum's TypeScript name.
type TSOpenNames struct {
	Known, IsKnown, Name string
}

// TSOpenEnumNames are the names opened enum e adds in TypeScript.
func TSOpenEnumNames(e *Enum) TSOpenNames {
	name := tsName(e)
	return TSOpenNames{Known: name + openKnown, IsKnown: tsIsPrefix + name + openKnown, Name: tsLowerCamel(name) + tsNameSuffix}
}

// TSFinderName is the container method of a @stable field f, findBy + UpperCamel of f's Canon name; a @ts(name:) override renames the row property, not the method (CODEGEN.md §5.9, DECISIONS 369).
func TSFinderName(f *Field) string { return tsFindByPrefix + cppUpperCamel(f.Name) }

// declareFinders declares, in the container of table or keyed list v, each @stable field's findBy<F>: two of one name are E8005 (DECISIONS 369).
func (pl *tsNamePlan) declareFinders(v *Value) {
	if v.Type.Kind != types.Table && (v.Type.Kind != types.List || v.Type.KeyedBy == nil) || v.Type.Elem == nil {
		return
	}
	rec, ok := v.Type.Elem.Named.(*Record)
	if !ok {
		return
	}
	sc := pl.scope(tsContainerScope + v.Name)
	for _, f := range rec.Fields {
		if f.Stable {
			pl.declare(sc, TSFinderName(f), pl.origin(v.Name)+qnameSep+f.Name, f)
		}
	}
}

// declareOpenEnum declares the names an enum the ts emit opens adds (DECISIONS 362).
func (pl *tsNamePlan) declareOpenEnum(mod *nameScope, e *Enum) {
	if !OpenEnum(pl.p, pl.e, e) {
		return
	}
	n := TSOpenEnumNames(e)
	for _, name := range []string{n.Known, n.IsKnown, n.Name} {
		pl.declare(mod, name, e.QName(), e)
	}
}
