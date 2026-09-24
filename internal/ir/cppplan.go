package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// CppNamePlan is every C++ name gen/cpp declares for a package and its cpp emit in data mode, per scope (CODEGEN.md §3.3–§3.5, §5, §7.2): the namespace, detail, conformance, each class and enum; stage E reports E8005 and E8011 from its problems (decision 37), and across the packages emitted into one namespace; gen/cpp writes its names from the lookups.
type CppNamePlan struct {
	p       *Package
	e       *Emit
	values  []*Value
	holders map[any][]*Value // per class (a *Record, *Variant or *Case), the emitted values holding it
	namer
	ns      *nameScope     // the namespace's own names that other packages' headers share (§3.5)
	nsItems map[string]any // the item each namespace name was declared for
	shared  []cppShared
}

// cppShared is a name a header declares in the emit's namespace, or in its detail or conformance namespace, with what declared it.
type cppShared struct {
	scope, name, origin string
	item                any
	overload            bool // ToName, ToWire, Decode: overloads of another package's meet it legally
}

// PlanCppNames is the name plan of p's cpp emit e, with every problem found.
func PlanCppNames(p *Package, e *Emit) *CppNamePlan {
	pl := &CppNamePlan{p: p, e: e, holders: map[any][]*Value{}, nsItems: map[string]any{}, namer: newNamer(cppValidIdent)}
	for _, v := range p.Values {
		if e.Values == nil || slices.Contains(e.Values, v.Name) {
			pl.values = append(pl.values, v)
		}
	}
	pl.holdersOf()
	pl.declareAll()
	return pl
}

// Problems are the plan's problems, in generation order.
func (pl *CppNamePlan) Problems() []GoNameProblem { return pl.problems }

// cppUpperCamel is C++'s UpperCamel(x) (CODEGEN.md §3.2): Cap of every word.
func cppUpperCamel(name string) string {
	var b strings.Builder
	for _, w := range Words(name) {
		b.WriteString(strings.ToUpper(w[:1]) + strings.ToLower(w[1:]))
	}
	return b.String()
}

// cppVerbatim is a name as Canon spells it, with `_` when C++ reserves it (CODEGEN.md §3.4).
func cppVerbatim(name string) string {
	if CppReserved(name) {
		return name + underscore
	}
	return name
}

func cppOverride(n NameOptions, derived string) string {
	if n.Name != "" {
		return n.Name
	}
	return derived
}

// TypeName is a record's, enum's or variant's C++ name: its Canon name, or its @cpp(name:) (CODEGEN.md §3.3, §3.5); "" for another Type.
func (pl *CppNamePlan) TypeName(t Type) string {
	switch x := t.(type) {
	case *Record:
		return cppOverride(NameOptions{Name: x.Cpp.Name}, x.Name)
	case *Enum:
		return cppOverride(x.Cpp, x.Name)
	case *Variant:
		return cppOverride(x.Cpp, x.Name)
	}
	return ""
}

// CaseName is a case's class, T + UpperCamel(c), or its @cpp(name:) (CODEGEN.md §3.3).
func (pl *CppNamePlan) CaseName(v *Variant, c *Case) string {
	return cppOverride(NameOptions{Name: c.Cpp.Name}, pl.TypeName(v)+cppUpperCamel(c.Name))
}

// AsName is a case's accessor, As + UpperCamel(c) or As + its @cpp(name:) (CODEGEN.md §3.5).
func (pl *CppNamePlan) AsName(c *Case) string {
	return cppAsPrefix + cppOverride(NameOptions{Name: c.Cpp.Name}, cppUpperCamel(c.Name))
}

// KindName is a variant's kind enum, TKind (CODEGEN.md §5.5).
func (pl *CppNamePlan) KindName(v *Variant) string { return pl.TypeName(v) + GoKind }

// Enumerator is an enum member, verbatim or its @cpp(name:) (CODEGEN.md §3.3).
func (pl *CppNamePlan) Enumerator(m *EnumMember) string {
	return cppOverride(m.Cpp, cppVerbatim(m.Name))
}

// KindMember is a case's kind-enum member, verbatim or its @cpp(name:) (CODEGEN.md §3.5).
func (pl *CppNamePlan) KindMember(c *Case) string {
	return cppOverride(NameOptions{Name: c.Cpp.Name}, cppVerbatim(c.Name))
}

// ConstName is a constant, verbatim or its @cpp(name:) (CODEGEN.md §3.3).
func (pl *CppNamePlan) ConstName(c *Const) string { return cppOverride(c.Cpp, cppVerbatim(c.Name)) }

// ContainerName is a table's or keyed list's class, UpperCamel(v) (CODEGEN.md §5.9).
func (pl *CppNamePlan) ContainerName(v *Value) string { return cppUpperCamel(v.Name) }

// SchemaName is a value's schema constant, k + UpperCamel(v) + Schema (CODEGEN.md §3.3).
func (pl *CppNamePlan) SchemaName(v *Value) string {
	return cppConstPrefix + cppUpperCamel(v.Name) + goSchemaSuffix
}

// FieldGetter is a field's getter, Get + UpperCamel(f) or its @cpp(name:), then Key or Keys for a ref (CODEGEN.md §3.3, §5.8); the resolved getter is the name without them.
func (pl *CppNamePlan) FieldGetter(f *Field) (getter, resolved string) {
	resolved = cppOverride(NameOptions{Name: f.Cpp.Name}, GoGet+cppUpperCamel(f.Name))
	return resolved + cppKeySuffix(f.Type), resolved
}

// FnName is a method's or package fn's name, UpperCamel(fn) or its @cpp(name:) (CODEGEN.md §3.3).
func (pl *CppNamePlan) FnName(fn *ExportFn) string {
	return cppOverride(fn.Cpp, cppUpperCamel(fn.Name))
}

// Member is the storage `f_` of a field, stored fn or snapshot value (CODEGEN.md §7.2), from its Canon name.
func (pl *CppNamePlan) Member(canon string) string { return canon + underscore }

// PureName is a translated method's pure function in detail, <Class>_<fn> (CODEGEN.md §3.3).
func (pl *CppNamePlan) PureName(class string, fn *ExportFn) string {
	return class + underscore + fn.Name
}

// Upper is P, the UpperCamel of the package's last segment, which the snapshot, store, access struct and conformance entry point are named after (CODEGEN.md §3.3).
func (pl *CppNamePlan) Upper() string {
	segs := strings.Split(pl.p.Name, qnameSep)
	return cppUpperCamel(segs[len(segs)-1])
}

// cppKeySuffix is Key for a ref, Keys for a list of refs, "" else (CODEGEN.md §3.3).
func cppKeySuffix(t TypeRef) string {
	switch {
	case t.Kind == types.Ref:
		return cppKey
	case t.Kind == types.List && t.Elem != nil && t.Elem.Kind == types.Ref:
		return cppKeys
	}
	return ""
}

// holdersOf lists, per class, the emitted values holding it by value: through fields, lists, maps, optionals and stored results (gen/cpp's rule).
func (pl *CppNamePlan) holdersOf() {
	for _, v := range pl.values {
		root := v.Type
		if root.Kind != types.Record && root.Elem != nil {
			root = *root.Elem
		}
		if root.Named != nil {
			pl.hold(root.Named, v, map[any]bool{})
		}
	}
}

func (pl *CppNamePlan) hold(key any, v *Value, seen map[any]bool) {
	if seen[key] || !pl.ownClass(key) {
		return
	}
	seen[key] = true
	pl.holders[key] = append(pl.holders[key], v)
	for _, to := range cppHeld(key) {
		pl.hold(to, v, seen)
	}
}

// ownClass reports a record, variant or case with fields of this package.
func (pl *CppNamePlan) ownClass(key any) bool {
	switch k := key.(type) {
	case *Record:
		return k.Pkg == pl.p.Name
	case *Variant:
		return k.Pkg == pl.p.Name
	case *Case:
		return len(k.Fields) > 0
	}
	return false
}

// cppHeld are the classes a class holds by value: a variant its cases with fields, a record or case the records and variants of its fields and stored results.
func cppHeld(key any) []any {
	if v, ok := key.(*Variant); ok {
		return casesWithFields(v)
	}
	fields, fns := classBody(key)
	var out []any
	for _, f := range fields {
		out = cppClassesOf(f.Type, out)
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = cppClassesOf(fn.Result, out)
		}
	}
	return out
}

func cppClassesOf(t TypeRef, out []any) []any {
	if (t.Kind == types.Record || t.Kind == types.Variant) && t.Named != nil {
		return append(out, t.Named)
	}
	if t.Elem != nil && (t.Kind == types.List || t.Kind == types.Map || t.Kind == types.Optional) {
		return cppClassesOf(*t.Elem, out)
	}
	return out
}

// Resolves reports a ref of class that gets a resolved getter: every holder of the class resolves it into its target container, the holder being that container or both being @reload; else it is a key only (CODEGEN.md §5.8, §5.11; log-2026-09-24 "ir name plans + support plan").
func (pl *CppNamePlan) Resolves(t TypeRef, class any) bool {
	if t.Kind == types.List && t.Elem != nil {
		t = *t.Elem
	}
	r := t.Ref
	if t.Kind != types.Ref || r == nil || r.Coll != types.CollLet || r.Local || r.Value == "" || r.Pkg != pl.p.Name {
		return false
	}
	i := slices.IndexFunc(pl.values, func(v *Value) bool { return v.Name == r.Value })
	if i < 0 || pl.values[i].Type.Kind == types.Record {
		return false
	}
	return allResolve(pl.holders[class], pl.values[i])
}
