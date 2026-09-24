package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// declareAll declares the namespace's names in gen/cpp's order (CODEGEN.md §2.7, §3.5): enums, classes with their kind enums, constants, values, package fns, snapshot and store; then each enum's and class's members, detail and conformance.
func (pl *CppNamePlan) declareAll() {
	pl.ns = pl.scope(pl.e.Namespace)
	pl.declareNamespaceTypes()
	for _, c := range pl.p.Consts {
		pl.shareNS(pl.ConstName(c), c.Name, c)
	}
	for _, v := range pl.values {
		pl.shareNS(pl.SchemaName(v), v.Name, v)
		if v.Type.Kind != types.Record {
			pl.shareNS(pl.ContainerName(v), v.Name, v)
		}
	}
	for _, fn := range pl.p.Fns {
		if fn.Kind == FnTranslated {
			pl.shareNS(pl.FnName(fn), fn.Name, fn)
		}
	}
	if pl.reloads() {
		pl.shareNS(pl.Upper()+goSnapshotSuffix, pl.p.Name, nil)
		pl.shareNS(pl.Upper()+goStoreSuffix, pl.p.Name, nil)
	}
	pl.declareMembers()
	pl.declareDetail()
	pl.declareConformance()
}

// declareNamespaceTypes declares the enums with their helpers, ToName and ToWire once for them all (overloads, CODEGEN.md §5.2), then the classes with the kind enums of variants.
func (pl *CppNamePlan) declareNamespaceTypes() {
	for _, t := range pl.p.Types {
		if e, ok := t.(*Enum); ok {
			pl.declareEnum(pl.TypeName(e), e.QName(), e, e.Codes != nil)
		}
	}
	if slices.ContainsFunc(pl.p.Types, func(t Type) bool { _, isEnum := t.(*Enum); _, isVariant := t.(*Variant); return isEnum || isVariant }) {
		for _, n := range cppEnumOverloads {
			pl.declare(pl.ns, n, pl.p.Name, nil)
			pl.shared = append(pl.shared, cppShared{scope: pl.e.Namespace, name: n, origin: pl.p.Name, overload: true})
		}
	}
	for _, c := range pl.classes() {
		pl.shareNS(pl.className(c), pl.classOrigin(c), c)
		if v, ok := c.(*Variant); ok {
			pl.declareEnum(pl.KindName(v), v.QName(), v, false)
		}
	}
}

// shareNS declares a namespace name that the header writes, which other packages emitted into the namespace share (§3.5).
func (pl *CppNamePlan) shareNS(name, origin string, item any) {
	if _, seen := pl.nsItems[name]; !seen {
		pl.nsItems[name] = item
	}
	pl.declare(pl.ns, name, origin, item)
	pl.share(pl.e.Namespace, name, origin, item)
}

// share records a name other packages' headers can meet in scope, its origin qualified by the package.
func (pl *CppNamePlan) share(scope, name, origin string, item any) {
	if origin != pl.p.Name && !strings.HasPrefix(origin, pl.p.Name+qnameSep) {
		origin = pl.p.Name + qnameSep + origin
	}
	pl.shared = append(pl.shared, cppShared{scope: scope, name: name, origin: origin, item: item})
}

// declareEnum declares an enum, k<E>Members, <E>FromWire and, with @codes, <E>FromCode (CODEGEN.md §5.2), then its members.
func (pl *CppNamePlan) declareEnum(name, origin string, item any, codes bool) {
	pl.shareNS(name, origin, item)
	pl.shareNS(cppConstPrefix+name+goMembersSuffix, origin, item)
	pl.shareNS(name+cppFromWire, origin, item)
	if codes {
		pl.shareNS(name+goFromCodeSuffix, origin, item)
	}
	sc := pl.scope(name)
	switch x := item.(type) {
	case *Enum:
		for _, m := range x.Members {
			pl.declare(sc, pl.Enumerator(m), origin+qnameSep+m.Name, m)
		}
	case *Variant:
		for _, c := range x.Cases {
			pl.declare(sc, pl.KindMember(c), origin+qnameSep+c.Name, c)
		}
	}
}

// classes are the classes gen/cpp declares, in declaration order: each record, each variant then its cases with fields (log-2026-09-24, gen/cpp round 3).
func (pl *CppNamePlan) classes() []any { return packageClasses(pl.p) }

// packageClasses are p's records, and each variant followed by its cases with fields, in declaration order: the classes both generators write.
func packageClasses(p *Package) []any {
	var out []any
	for _, t := range p.Types {
		switch x := t.(type) {
		case *Record:
			out = append(out, x)
		case *Variant:
			out = append(append(out, x), casesWithFields(x)...)
		}
	}
	return out
}

func casesWithFields(v *Variant) []any {
	var out []any
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// className is a class's C++ name; a case's needs its variant.
func (pl *CppNamePlan) className(c any) string {
	if cs, ok := c.(*Case); ok {
		return pl.CaseName(pl.variantOf(cs), cs)
	}
	return pl.TypeName(c.(Type))
}

func (pl *CppNamePlan) variantOf(c *Case) *Variant {
	for _, t := range pl.p.Types {
		if v, ok := t.(*Variant); ok && slices.Contains(v.Cases, c) {
			return v
		}
	}
	return &Variant{}
}

// classOrigin is a class as messages name it: its qualified name, a case's under its variant's.
func (pl *CppNamePlan) classOrigin(c any) string {
	if cs, ok := c.(*Case); ok {
		return pl.variantOf(cs).QName() + qnameSep + cs.Name
	}
	return c.(Type).QName()
}

func (pl *CppNamePlan) reloads() bool {
	for _, v := range pl.values {
		if v.Reload {
			return true
		}
	}
	return false
}
