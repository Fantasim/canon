package ir

import "strings"

// BoundName is a bound constant of field f, a static member of its class: k + <F> + the path and side, <F> what follows Get in the field's getter, its override included (CODEGEN.md §5.15, §3.3).
func (pl *CppNamePlan) BoundName(f *Field, b BoundConst) string {
	_, resolved := pl.FieldGetter(f)
	return cppConstPrefix + strings.TrimPrefix(resolved, GoGet) + boundSuffix(b)
}

// declareBounds reserves each bound constant of f in its class, written or not (CODEGEN.md §5.15).
func (pl *CppNamePlan) declareBounds(sc *nameScope, f *Field, origin string) {
	for _, b := range FieldBounds(f) {
		pl.declare(sc, pl.BoundName(f, b), origin, f)
	}
}

// BoundName is a bound constant of field f of the Go type owner: owner_ + the field's getter + the path and side (CODEGEN.md §5.15, §3.3).
func (pl *GoNamePlan) BoundName(owner string, f *Field, b BoundConst) string {
	return owner + underscore + goExported(f.Go, f.Name) + boundSuffix(b)
}

// declareBounds reserves, in the package scope, each bound constant of the fields of the type owner names, built on the type's name (CODEGEN.md §5.15).
func (pl *GoNamePlan) declareBounds(top *nameScope, owner string, st goStruct, fields []*Field) {
	for _, f := range fields {
		for _, b := range FieldBounds(f) {
			build := func() string { return pl.BoundName(st.goName(), f, b) }
			pl.declareFrom(top, owner+qnameSep+f.Name, f, derivation{st.self, build})
		}
	}
}

// TSBoundName is a bound constant of field f of the TypeScript type owner: owner_ + UpperCamel of its property + the path and side (CODEGEN.md §5.15, §3.3).
func TSBoundName(owner string, f *Field, b BoundConst) string {
	return owner + underscore + cppUpperCamel(tsEffective(f.TS, f.Name)) + boundSuffix(b)
}

// declareBounds reserves, in the module scope, each bound constant of the fields of the type owner; TypeScript has no inputs (CODEGEN.md §5.15).
func (pl *tsNamePlan) declareBounds(mod *nameScope, owner, origin string, fields []*Field) {
	for _, f := range fields {
		if f.Input != nil {
			continue
		}
		for _, b := range FieldBounds(f) {
			pl.declare(mod, TSBoundName(owner, f, b), origin+qnameSep+f.Name, f)
		}
	}
}
