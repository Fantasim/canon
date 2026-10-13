package ir

import "slices"

// OpenEnums are the enums of the package the cpp emit opens, in declaration order (DECISIONS 362): gen/cpp writes each as a class holding its wire.
func (pl *CppNamePlan) OpenEnums() []*Enum {
	var out []*Enum
	for _, t := range pl.p.Types {
		if e, ok := t.(*Enum); ok && OpenEnum(pl.p, pl.e, e) {
			out = append(out, e)
		}
	}
	return out
}

// declareOpenClass declares, in an opened enum's class scope, what its enumerators meet: the class's own name (its constructor) and its wire storage, so that an enumerator named like either is E8005 (CODEGEN.md §5.2, DECISIONS 362).
func (pl *CppNamePlan) declareOpenClass(sc *nameScope, name, origin string, e *Enum) {
	if !OpenEnum(pl.p, pl.e, e) {
		return
	}
	pl.declare(sc, name, origin, nil)
	pl.declare(sc, cppOpenWire, origin, nil)
}

// openOverloads are the free functions every opened enum of the package adds, one overload each: IsKnown, and ToCode when one has @codes (CODEGEN.md §5.2, DECISIONS 362).
func (pl *CppNamePlan) openOverloads() []string {
	open := pl.OpenEnums()
	if len(open) == 0 {
		return nil
	}
	if slices.ContainsFunc(open, func(e *Enum) bool { return e.Codes != nil }) {
		return []string{cppIsKnown, cppToCode}
	}
	return []string{cppIsKnown}
}
