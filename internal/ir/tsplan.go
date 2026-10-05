package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// tsNamePlan is the names of a ts emit's module and interfaces, with every problem found: stage E checks them as gen/ts writes them (CODEGEN.md §3.3–§3.5, DECISIONS 278).
type tsNamePlan struct {
	namer
	p       *Package
	e       *Emit
	values  []*Value
	entries map[*Record]bool
}

// planTSNames is the name plan of p's ts emit e.
func planTSNames(p *Package, e *Emit) *tsNamePlan {
	pl := &tsNamePlan{p: p, e: e, namer: newNamer(p.Name, tsValidIdent), values: tsSelected(p, e)}
	pl.indexEntries()
	pl.declareAll()
	return pl
}

// found are the plan's problems, in generation order.
func (pl *tsNamePlan) found() []GoNameProblem { return pl.problems }

// TSHelperNames are the names of the helper units a ts module may hold (CODEGEN.md §8.2): no declaration of the package may take one.
func TSHelperNames() []string { return slices.Clone(tsHelpers) }

// TSReservedWords are the names a TypeScript top-level binding or parameter may not take (CODEGEN.md §3.4): gen/ts escapes them with `_`.
func TSReservedWords() []string { return slices.Clone(tsReserved) }

func tsValidIdent(name string) (ok, reserved bool) { return tsIdentPattern.MatchString(name), false }

// tsEscape suffixes `_` to a name reserved at top level or as a parameter (CODEGEN.md §3.4).
func tsEscape(name string) string {
	if slices.Contains(tsReserved, name) {
		return name + underscore
	}
	return name
}

// tsLowerCamel is lowerCamel(x) (CODEGEN.md §3.2).
func tsLowerCamel(name string) string {
	ws := words(name)
	if len(ws) == 0 {
		return ""
	}
	return strings.ToLower(ws[0]) + cppUpperCamel(strings.Join(ws[1:], underscore))
}

// tsTypeName is a type's name: its first letter upper-cased, or the override (CODEGEN.md §3.3).
func tsTypeName(n NameOptions, name string) string {
	if n.Name != "" || name == "" {
		return n.Name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func tsEffective(n NameOptions, name string) string {
	if n.Name != "" {
		return n.Name
	}
	return name
}

// tsIDName is the id type of a table of rec, <Rec>Id (CODEGEN.md §5.3).
func tsIDName(rec *Record) string { return tsName(rec) + tsIDSuffix }

// tsName is the TypeScript name of a named type.
func tsName(t Type) string {
	switch x := t.(type) {
	case *Record:
		return tsTypeName(x.TS, x.Name)
	case *Enum:
		return tsTypeName(x.TS, x.Name)
	case *Variant:
		return tsTypeName(x.TS, x.Name)
	case *Dependent:
		return tsTypeName(x.TS, x.Name)
	}
	return ""
}

// tsCaseName is a case's type, T + UpperCamel(c), or its override.
func tsCaseName(v *Variant, c *Case) string {
	return tsEffective(c.TS, tsName(v)+cppUpperCamel(c.Name))
}

// indexEntries finds the records that are the rows of a table: they hold `id` and `retired`.
func (pl *tsNamePlan) indexEntries() {
	pl.entries, _ = tsRows(pl.p, pl.values)
}

// TSRows are the records a ts emit writes as table rows, holding `id` and `retired`, and those of them it also writes as plain values elsewhere, where both are optional (CODEGEN.md §5.3, §5.4, DECISIONS 278); gen/ts writes from them.
func TSRows(p *Package, e *Emit) (rows, loose map[*Record]bool) {
	return tsRows(p, tsSelected(p, e))
}

// tsSelected are the values a ts emit writes: `values`, or every value; none in types mode.
func tsSelected(p *Package, e *Emit) []*Value {
	var out []*Value
	for _, v := range p.Values {
		if e.Mode != ModeTypes && (len(e.Values) == 0 || slices.Contains(e.Values, v.Name)) {
			out = append(out, v)
		}
	}
	return out
}

// tsRows finds the rows of the tables values and fields hold, then the rows also held in a plain position: a field, a list element, a value, a constant or a stored result, not as a table's element.
func tsRows(p *Package, values []*Value) (rows, loose map[*Record]bool) {
	rows, loose = map[*Record]bool{}, map[*Record]bool{}
	mark := func(t TypeRef) {
		if t.Kind == types.Table && t.Elem != nil {
			if r, ok := t.Elem.Named.(*Record); ok {
				rows[r] = true
			}
		}
	}
	var held []TypeRef
	for _, v := range values {
		held = append(held, v.Type)
	}
	for _, class := range packageClasses(p) {
		fields, _ := classBody(class)
		for _, f := range fields {
			held = append(held, f.Type)
		}
	}
	for _, t := range held {
		walkTypeRef(t, mark)
	}
	for _, t := range append(held, tsResultTypes(p)...) {
		markPlain(t, rows, loose)
	}
	return rows, loose
}

// tsResultTypes are the types of the package's constants and of its stored fns' results, methods' included.
func tsResultTypes(p *Package) []TypeRef {
	var out []TypeRef
	for _, c := range p.Consts {
		out = append(out, c.Type)
	}
	for _, fn := range p.Fns {
		out = append(out, fn.Result)
	}
	for _, class := range tsClasses(p) {
		_, fns := classBody(class)
		for _, fn := range fns {
			out = append(out, fn.Result)
		}
	}
	return out
}

// markPlain marks loose every row record t holds outside a table's element.
func markPlain(t TypeRef, rows, loose map[*Record]bool) {
	if r, ok := t.Named.(*Record); ok && t.Kind == types.Record && rows[r] {
		loose[r] = true
	}
	if t.Elem != nil && t.Kind != types.Table {
		markPlain(*t.Elem, rows, loose)
	}
	if t.Key != nil {
		markPlain(*t.Key, rows, loose)
	}
}

// walkTypeRef calls visit on t and every type it holds.
func walkTypeRef(t TypeRef, visit func(TypeRef)) {
	visit(t)
	if t.Elem != nil {
		walkTypeRef(*t.Elem, visit)
	}
	if t.Key != nil {
		walkTypeRef(*t.Key, visit)
	}
}

func (pl *tsNamePlan) origin(name string) string { return pl.p.Name + qnameSep + name }

// declareAll declares every name in gen/ts's order: helpers, the names it may import, constants, enums, kind and branch unions, id types, types, values, fns, then each interface's properties.
func (pl *tsNamePlan) declareAll() {
	mod := pl.scope(tsModuleScope)
	for _, h := range tsHelpers { // every helper name, used or not (DECISIONS 278)
		pl.declare(mod, h, tsHelperOrigin, nil)
	}
	pl.declareImports(mod)
	for _, c := range pl.p.Consts {
		pl.declare(mod, tsEscape(tsEffective(c.TS, c.Name)), pl.origin(c.Name), c)
	}
	for _, t := range pl.p.Types {
		pl.declareType(mod, t)
	}
	for _, v := range pl.values {
		pl.declareValue(mod, v)
	}
	for _, fn := range pl.p.Fns {
		pl.declareFn(mod, fn)
	}
	pl.declareMethods(mod)
	for _, t := range pl.p.Types {
		pl.declareProps(t)
	}
	pl.declareForeignRows()
}

// declareForeignRows declares the properties of CanonRow<T, K> for each record of another package a table of this file holds: its `id` and `retired`, then the record's own, fields and stored fns alike, so a property `id` or `retired` is E8005 at the table (log-2026-10-06 "U4 (gen/ts) done" (g), "U1 rounds 3-5" 1).
func (pl *tsNamePlan) declareForeignRows() {
	for _, row := range ForeignRows(pl.p, pl.e) {
		holder, org := rowHolder(row), row.Record.QName()
		sc := pl.scope(tsCanonRowScope + org)
		pl.declare(sc, tsIDProp, tsCanonRowScope+org, holder)
		pl.declare(sc, tsRetiredProp, tsCanonRowScope+org, holder)
		for _, f := range row.Record.Fields {
			if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) {
				pl.declare(sc, tsEffective(f.TS, f.Name), org+qnameSep+f.Name, holder)
			}
		}
		for _, fn := range row.Record.Methods {
			if fn.Kind != FnTranslated {
				pl.declare(sc, tsEffective(fn.TS, fn.Name), org+qnameSep+fn.Name, holder)
			}
		}
	}
}

// declareType declares a type's names: itself, its enum tables, kind and branch unions, case types, readers, and in types mode its decode and parse functions (CODEGEN.md §8.1).
func (pl *tsNamePlan) declareType(mod *nameScope, t Type) {
	name, org := tsName(t), t.QName()
	pl.declare(mod, name, org, t)
	switch x := t.(type) {
	case *Enum:
		pl.declareEnumTables(mod, x, name)
	case *Variant:
		pl.declare(mod, name+tsKindSuffix, org, t)
		pl.declareCases(mod, x, org)
	case *Dependent:
		pl.declare(mod, name+tsBranchSuffix, org, t)
	}
	if _, enum := t.(*Enum); !enum {
		pl.declareRead(mod, name, org, t)
	}
	if (isRecord(t) || isVariant(t)) && pl.e.Mode == ModeTypes {
		pl.declare(mod, tsDecodePrefix+name, org, t)
		pl.declare(mod, tsParsePrefix+name, org, t)
	}
}

// declareEnumTables declares an enum's Members, Names and Index, and Codes with @codes.
func (pl *tsNamePlan) declareEnumTables(mod *nameScope, e *Enum, name string) {
	for _, suffix := range []string{tsMembersName, tsNamesName, tsIndexName} {
		pl.declare(mod, name+suffix, e.QName(), e)
	}
	if e.Codes != nil {
		pl.declare(mod, name+tsCodesName, e.QName(), e)
	}
}

// declareCases declares the type and the reader of each case with fields.
func (pl *tsNamePlan) declareCases(mod *nameScope, v *Variant, org string) {
	for _, c := range v.Cases {
		if len(c.Fields) > 0 || len(c.Methods) > 0 {
			pl.declare(mod, tsCaseName(v, c), org+qnameSep+c.Name, c)
			pl.declareRead(mod, tsCaseName(v, c), org+qnameSep+c.Name, c)
		}
	}
}

// declareRead declares the private reader of a class in a mode that decodes.
func (pl *tsNamePlan) declareRead(mod *nameScope, name, origin string, item any) {
	if pl.e.Mode == ModeData || pl.e.Mode == ModeTypes {
		pl.declare(mod, tsReadPrefix+name, origin, item)
	}
}

// declareValue declares an emitted value: its accessor, or in data mode its schema and its decode and parse functions; a table's id type and Index.
func (pl *tsNamePlan) declareValue(mod *nameScope, v *Value) {
	org, eff := pl.origin(v.Name), tsEffective(v.TS, v.Name)
	if v.Type.Kind == types.Table && v.Type.Elem != nil && v.Type.Elem.Named != nil {
		id := tsName(v.Type.Elem.Named) + tsIDSuffix
		pl.declare(mod, id, org, v)
		if pl.e.Mode != ModeData {
			pl.declare(mod, id+tsIndexName, org, v)
		}
	}
	if pl.e.Mode != ModeData {
		pl.declare(mod, tsEscape(eff), org, v)
		return
	}
	pl.declare(mod, eff+tsSchemaSuffix, org, v)
	pl.declare(mod, tsDecodePrefix+cppUpperCamel(eff), org, v)
	pl.declare(mod, tsParsePrefix+cppUpperCamel(eff), org, v)
}

// declareFn declares a package-level fn: its function, and the table or frozen value it reads.
func (pl *tsNamePlan) declareFn(mod *nameScope, fn *ExportFn) {
	org := pl.fnOrigin(fn)
	pl.declare(mod, tsEscape(tsEffective(fn.TS, tsLowerCamel(fn.Name))), org, fn)
	if fn.Kind == FnLookup || fn.Kind == FnPrecomputed && tsComposite(fn.Result) {
		pl.declare(mod, strings.ToUpper(strings.Join(words(fn.Name), underscore)), org, fn)
	}
}

// tsComposite reports a result written as a frozen composite value.
func tsComposite(t TypeRef) bool {
	switch t.Kind {
	case types.Record, types.Variant, types.Case, types.List:
		return true
	default:
		return false
	}
}

// declareMethods declares each translated method's public function and its pure `$` function.
func (pl *tsNamePlan) declareMethods(mod *nameScope) {
	for _, t := range pl.p.Types {
		switch x := t.(type) {
		case *Record:
			pl.declareMethodNames(mod, x.Name, x.Methods, x.QName())
		case *Variant:
			for _, c := range x.Cases {
				pl.declareMethodNames(mod, x.Name+underscore+c.Name, c.Methods, x.QName()+qnameSep+c.Name)
			}
		}
	}
}

func (pl *tsNamePlan) declareMethodNames(mod *nameScope, owner string, fns []*ExportFn, org string) {
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			continue
		}
		public := tsEscape(tsEffective(fn.TS, tsLowerCamel(owner)+cppUpperCamel(fn.Name)))
		pl.declare(mod, public, org+qnameSep+fn.Name, fn)
		pl.declare(mod, tsPureMark+public, org+qnameSep+fn.Name, fn)
	}
}

// declareProps declares the properties of a record's or case's interface: `id` and `retired` of a table row, the fields, the stored fns; a case's `kind` too.
func (pl *tsNamePlan) declareProps(t Type) {
	switch x := t.(type) {
	case *Record:
		sc := pl.scope(x.QName())
		if pl.entries[x] {
			pl.declare(sc, tsIDProp, x.QName(), x)
			pl.declare(sc, tsRetiredProp, x.QName(), x)
		}
		pl.declareBody(sc, x.QName(), x.Fields, x.Methods)
	case *Variant:
		for _, c := range x.Cases {
			sc := pl.scope(x.QName() + qnameSep + c.Name)
			pl.declare(sc, tsKindProp, x.QName()+qnameSep+c.Name, c)
			pl.declareBody(sc, x.QName()+qnameSep+c.Name, c.Fields, c.Methods)
		}
	}
}

func (pl *tsNamePlan) declareBody(sc *nameScope, org string, fields []*Field, fns []*ExportFn) {
	for _, f := range fields {
		if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) {
			pl.declare(sc, tsEffective(f.TS, f.Name), org+qnameSep+f.Name, f)
		}
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			pl.declare(sc, tsEffective(fn.TS, fn.Name), org+qnameSep+fn.Name, fn)
		}
	}
}

func isVariant(t Type) bool {
	_, ok := t.(*Variant)
	return ok
}
