package tsgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// records writes records, variants and dependent types in declaration order (CODEGEN.md §2.7, §5.4–§5.6).
func (g *gen) records() {
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			g.record(x)
		case *ir.Variant:
			g.variant(x)
		case *ir.Dependent:
			g.dependent(x)
		}
	}
}

// record is an interface of readonly properties: `id` and `retired` for a table's rows, the fields, then the precomputed fns (CODEGEN.md §5.4).
func (g *gen) record(r *ir.Record) {
	defer g.enter(r.QName())()
	name := g.declare(typeName(r), r.QName())
	var props []string
	if g.entries[r] {
		props = append(props, g.entryProps(r)...)
	}
	props = append(props, g.bodyProps(r.QName(), r.Fields, r.Methods)...)
	g.add(docComment("", r.Doc) + g.interfaceText(name, props))
}

// entryProps are the `id` and `retired` properties of a table row, optional when the record is also a plain value (CODEGEN.md §5.3, §5.4).
func (g *gen) entryProps(r *ir.Record) []string {
	id := g.idTypeOf(r)
	mark := ""
	if g.loose[r] {
		mark = optionalMark
	}
	return []string{
		fmt.Sprintf(propFormat, g.declareProp(r.QName(), idProp)+mark, id),
		fmt.Sprintf(propFormat, g.declareProp(r.QName(), retiredProp)+mark, tsBoolean),
	}
}

// idTypeOf is the type of a row's id: the table's id type when this emit declares one and no other table holds the record, else string.
func (g *gen) idTypeOf(r *ir.Record) string {
	if g.tableOf(r) != nil && g.rowSites[r] == 1 {
		return idName(r)
	}
	return tsString
}

// bodyProps are the properties of a record or case: fields in order, then precomputed and finite-parameter fns.
func (g *gen) bodyProps(owner string, fields []*ir.Field, fns []*ir.ExportFn) []string {
	var props []string
	for _, f := range fields {
		if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
			continue
		}
		props = append(props, docComment(indent, f.Doc)+fmt.Sprintf(propFormat, g.declareProp(owner, fieldProp(f)), g.fieldType(f)))
	}
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			props = append(props, docComment(indent, fn.Doc)+fmt.Sprintf(propFormat, g.declareProp(owner, fnProp(fn)), g.fnType(fn)))
		}
	}
	return props
}

// declareProp checks that a property is unique in its interface.
func (g *gen) declareProp(owner, name string) string {
	key := owner + dot + name
	if g.props[key] {
		g.failf(errCollision, collisionFormat, key, key, name)
	}
	g.props[key] = true
	return name
}

// fieldProp is a field's property name: the Canon name, or its @ts(name:) override (CODEGEN.md §3.3).
func fieldProp(f *ir.Field) string { return effective(f.TS, f.Name) }

// fnProp is a precomputed or finite fn's property name.
func fnProp(fn *ir.ExportFn) string { return effective(fn.TS, fn.Name) }

// fieldType is the type of a field's property; an optional field is `T | null`.
func (g *gen) fieldType(f *ir.Field) string {
	t := g.tsType(f.Type, f.BigInt)
	if f.Optional {
		return t + unionSep + tsNull
	}
	return t
}

// fnType is the type of a precomputed fn's property, or of a finite fn's frozen table (CODEGEN.md §5.10).
func (g *gen) fnType(fn *ir.ExportFn) string {
	t := g.tsType(fn.Result, false)
	for _, p := range slices.Backward(fn.Params) {
		t = fmt.Sprintf(readonlyRecordFormat, g.domainType(p.Type), t)
	}
	return t
}

// domainType is the type of a finite parameter's keys: an enum, a table's ids, or `"false" | "true"` for a Bool.
func (g *gen) domainType(t ir.TypeRef) string {
	if t.Kind == types.Bool {
		return boolKeys
	}
	return g.keyType(t, false)
}

// interfaceText is `export interface name { props }`.
func (g *gen) interfaceText(name string, props []string) string {
	if len(props) == 0 {
		return fmt.Sprintf(emptyInterfaceFormat, name)
	}
	return fmt.Sprintf(interfaceFormat, name, strings.Join(props, ""))
}

// variant writes one interface per case with fields, then the union (CODEGEN.md §5.5).
func (g *gen) variant(v *ir.Variant) {
	defer g.enter(v.QName())()
	g.declare(typeName(v), v.QName())
	arms := make([]string, len(v.Cases))
	for i, c := range v.Cases {
		arms[i] = g.caseArm(v, c)
	}
	g.add(docComment("", v.Doc) + fmt.Sprintf(unionFormat, typeName(v), strings.Join(arms, unionSep)))
}

// caseArm writes a case's interface (when it has fields) and returns its place in the variant's union.
func (g *gen) caseArm(v *ir.Variant, c *ir.Case) string {
	if !hasInterface(c) {
		return fmt.Sprintf(bareCaseFormat, quote(c.Wire))
	}
	name := g.declare(caseName(v, c), v.QName()+dot+c.Name)
	props := []string{fmt.Sprintf(propFormat, kindProp, quote(c.Wire))}
	props = append(props, g.bodyProps(name, c.Fields, c.Methods)...)
	g.add(docComment("", c.Doc) + g.interfaceText(name, props))
	return name
}

// dependent is the union of a dependent type's branches, discriminated by `branch` (CODEGEN.md §5.6).
func (g *gen) dependent(d *ir.Dependent) {
	defer g.enter(d.QName())()
	name := g.declare(typeName(d), d.QName())
	if len(d.Branches) == 0 {
		g.failf(ErrMalformed, malformedDependent, d.QName())
		return
	}
	arms := make([]string, len(d.Branches))
	for i, b := range d.Branches {
		arms[i] = fmt.Sprintf(branchArmFormat, quote(b.Name), g.tsType(b.Type, false))
	}
	g.add(docComment("", d.Doc) + fmt.Sprintf(dependentFormat, name, strings.Join(arms, newline)))
}

// hasInterface reports a case written as its own interface, with its own reader: one with fields or export fns (CODEGEN.md §5.5).
func hasInterface(c *ir.Case) bool { return len(c.Fields) > 0 || len(c.Methods) > 0 }
