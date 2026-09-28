package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// declareAll declares the namespace's names in gen/cpp's order (CODEGEN.md §2.7, §3.5): enums, classes with their kind enums, constants, values, package fns, snapshot and store; then each enum's and class's members, detail and conformance. LoadInputs and its helpers come first, so a clash is reported at the user's item.
func (pl *CppNamePlan) declareAll() {
	pl.ns = pl.scope(pl.e.Namespace)
	pl.declareInputNames()
	pl.declareNamespaceTypes()
	for _, c := range pl.p.Consts {
		pl.shareNS(pl.ConstName(c), c.Name, c)
	}
	for _, v := range pl.values {
		pl.shareNS(pl.SchemaName(v), pl.valueOrigin(v), v)
		if v.Type.Kind != types.Record {
			pl.shareNS(pl.ContainerName(v), pl.valueOrigin(v), v)
		}
	}
	for _, fn := range pl.p.Fns {
		if fn.Kind == FnTranslated {
			pl.shareNS(pl.FnName(fn), pl.fnOrigin(fn), fn)
		}
	}
	if pl.reloads() {
		pl.shareNS(pl.SnapshotName(), pl.p.Name, nil)
		pl.shareNS(pl.StoreName(), pl.p.Name, nil)
	}
	pl.declareMembers()
	pl.checkSignatures()
	pl.declareDetail()
	pl.declareConformance()
}

// declareNamespaceTypes declares the enums with their helpers, ToName and ToWire once for them all (overloads, CODEGEN.md §5.2), the dependent types (§5.6), then the classes with the kind enums of variants.
func (pl *CppNamePlan) declareNamespaceTypes() {
	for _, t := range pl.p.Types {
		if e, ok := t.(*Enum); ok {
			pl.declareEnum(pl.TypeName(e), e.QName(), e, e.Codes != nil)
		}
	}
	if slices.ContainsFunc(pl.p.Types, func(t Type) bool { _, isEnum := t.(*Enum); _, isVariant := t.(*Variant); return isEnum || isVariant }) {
		for _, n := range cppEnumOverloads {
			pl.declare(pl.ns, n, pl.p.Name, nil)
			pl.shareAs(pl.e.Namespace, n, pl.p.Name, nil, meetsOverload)
		}
	}
	pl.declareDependents()
	for _, c := range pl.classes() {
		pl.shareNSFrom(pl.classOrigin(c), c, derivation{pl.classFrom(c), func() string { return pl.className(c) }})
		if v, ok := c.(*Variant); ok {
			pl.declareEnum(pl.KindName(v), v.QName(), v, false)
		}
	}
}

// shareNS declares a namespace name that the header writes, which other packages emitted into the namespace share (§3.5).
func (pl *CppNamePlan) shareNS(name, origin string, item any) {
	pl.shareNSFrom(origin, item, derivation{build: func() string { return name }})
}

// shareNSFrom is shareNS for a name d builds on the name of a type (declareFrom).
func (pl *CppNamePlan) shareNSFrom(origin string, item any, d derivation) {
	name := d.build()
	if _, seen := pl.nsItems[name]; !seen {
		pl.nsItems[name] = item
	}
	pl.declareFrom(pl.ns, origin, item, d)
	pl.share(pl.e.Namespace, name, origin, item)
}

// share records a name other packages' headers can meet in scope, its origin qualified by the package.
func (pl *CppNamePlan) share(scope, name, origin string, item any) {
	pl.shareAs(scope, name, origin, item, meetsNever)
}

// shareAs is share for a name that meets another package's same name of kind meets legally.
func (pl *CppNamePlan) shareAs(scope, name, origin string, item any, meets cppMeet) {
	if origin != pl.p.Name && !strings.HasPrefix(origin, pl.p.Name+qnameSep) {
		origin = pl.p.Name + qnameSep + origin
	}
	pl.shared = append(pl.shared, cppShared{scope: scope, name: name, origin: origin, item: item, meets: meets})
}

// shareSegments shares each segment of the emit's namespace in its parent: a class or any other name of another package there collides with the namespace (CODEGEN.md §3.5; g++: "redeclared as different kind of entity").
func (pl *CppNamePlan) shareSegments() {
	segs := strings.Split(pl.e.Namespace, cppScope)
	for i := range segs {
		pl.shareAs(strings.Join(segs[:i], cppScope), segs[i], pl.p.Name, nil, meetsNamespace)
	}
}

// declareEnum declares an enum, k<E>Members, <E>FromWire and, with @codes, <E>FromCode (CODEGEN.md §5.2), then its members.
func (pl *CppNamePlan) declareEnum(name, origin string, item any, codes bool) {
	h := pl.EnumHelpers(name)
	pl.shareNS(name, origin, item)
	pl.shareNS(h.Members, origin, item)
	pl.shareNS(h.FromWire, origin, item)
	if codes {
		pl.shareNS(h.FromCode, origin, item)
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

// classFrom is the variant a case's class name is built on, nil for a record or variant, or a case whose @cpp(name:) replaces it (CODEGEN.md §3.3).
func (pl *CppNamePlan) classFrom(c any) any {
	if cs, ok := c.(*Case); ok {
		return unlessOverridden(NameOptions{Name: cs.Cpp.Name}, pl.variantOf(cs))
	}
	return nil
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
