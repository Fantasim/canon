package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// nameScope is one namespace of generated code and what declared each of its names (§3.5); hidden are the names its body uses that a name declared in it would hide, with what declared them.
type nameScope struct {
	what          string
	names, hidden map[string]string
	items         map[string]any  // the item each name was declared for, nil for a fixed name
	hooks         map[string]bool // the names declared for make hooks (declareHook)
}

// namer is a name plan's scopes and problems; ident tells an identifier of the target, and
// whether an invalid one fails for a reserved reason (E8011 `reserved`) rather than for not
// being a plain identifier at all (E8011 `derived`).
type namer struct {
	scopes   []*nameScope
	problems []GoNameProblem
	macros   map[string]bool     // cpp only: the names common platform headers define as macros (CODEGEN.md §3.5); nil for Go
	warnings []GoNameProblem     // W8006: a declared name found in macros, never a problem (generation proceeds)
	warned   map[originPair]bool // the (name, origin) pairs already warned
	reported map[originPair]bool // the pairs of origins already reported colliding: one E8005 per cause (decision 203)
	derived  map[any]bool        // the items whose name, built on a refused type's, is that type's E8011 (decision 213)
	ident    func(string) (ok, reserved bool)
	self     string // the package: what an item meeting its own name is reported against
	bare     bool   // while a derived name is rebuilt on the default type names: every type's and case's override is ignored (decision 213)
}

// derivation is a name built on the name of type from, and how to build it: rebuilt bare, it tells a name refused through from from one refused for the item's own reason (decision 213).
type derivation struct {
	from  any
	build func() string
}

// typeOverride is a type's or case's own name options, none while a name is rebuilt bare.
func (n *namer) typeOverride(o NameOptions) NameOptions {
	if n.bare {
		return NameOptions{}
	}
	return o
}

func newNamer(pkg string, ident func(string) (ok, reserved bool)) namer {
	return namer{reported: map[originPair]bool{}, warned: map[originPair]bool{}, derived: map[any]bool{}, ident: ident, self: pkg}
}

// fnOrigin is a package-level export fn's origin, qualified as a method's is (a.echo, like a.Item.mix).
func (n *namer) fnOrigin(fn *ExportFn) string { return n.self + qnameSep + fn.Name }

// scope opens a new scope of the plan.
func (n *namer) scope(what string) *nameScope {
	sc := &nameScope{what: what, names: map[string]string{}}
	n.scopes = append(n.scopes, sc)
	return sc
}

// declare adds name to sc for origin; a name that is no identifier of the target (decision 202), or that sc already holds, is a problem. Two origins collide once in the whole plan, however many of their names meet, in any scope (decision 203).
func (n *namer) declare(sc *nameScope, name, origin string, item any) {
	if ok, reserved := n.ident(name); !ok {
		n.problems = append(n.problems, GoNameProblem{Kind: GoNotIdentifier, Scope: sc.what, Name: name, Origin: origin, Item: item, Reserved: reserved})
		return
	}
	if first, ok := sc.names[name]; ok {
		n.collide(sc, name, first, origin, item)
		return
	}
	sc.names[name] = origin
	if sc.items == nil {
		sc.items = map[string]any{}
	}
	sc.items[name] = item
	n.warnMacro(sc, name, origin, item)
	if first, ok := sc.hidden[name]; ok {
		n.collide(sc, name, first, origin, item)
	}
}

// collide is E8005 for name, declared in sc for first and for origin at item, once per pair of origins as declared; an item
// meeting a name of its own (a container's member named like the container) is reported against the package's generated
// code, so no message names one thing twice, yet stays apart from first meeting a name the package itself declares.
func (n *namer) collide(sc *nameScope, name, first, origin string, item any) {
	pair := originPair{first, origin}
	if n.reported[pair] {
		return
	}
	n.reported[pair] = true
	if origin == first {
		origin = n.self
	}
	n.problems = append(n.problems, GoNameProblem{Kind: GoCollision, Scope: sc.what, Name: name, First: first, Origin: origin, Item: item, FirstItem: sc.items[name]})
}

// declareFrom is declare for a name d builds on the name of a type (a member constant, a case type, a table id, a method's generated helper), unless it is that type's E8011 (builtOnRefused).
func (n *namer) declareFrom(sc *nameScope, origin string, item any, d derivation) {
	if name := d.build(); !n.builtOnRefused(name, item, d) {
		n.declare(sc, name, origin, item)
	}
}

// builtOnRefused reports a name of item, built by d on the name of a type refused already, that is no identifier while the same name built on the default type names is one: it is the type's E8011, not item's, and is not declared (decision 213, CODEGEN.md §3.5; provenance decides, never a text prefix). item is then refused through the type, so a name built on item's in turn (a case type's method) is the type's too.
func (n *namer) builtOnRefused(name string, item any, d derivation) bool {
	if d.from == nil || !n.refused(d.from) {
		return false
	}
	if ok, _ := n.ident(name); ok {
		return false
	}
	n.bare = true
	def := d.build()
	n.bare = false
	if ok, _ := n.ident(def); !ok {
		return false
	}
	n.derived[item] = true
	return true
}

// refused reports an item whose own name is already a problem other than a collision (an invalid override, or a name that is no identifier), or one through the type it is built on (builtOnRefused).
func (n *namer) refused(item any) bool {
	return n.derived[item] || slices.ContainsFunc(n.problems, func(pr GoNameProblem) bool { return pr.Kind != GoCollision && pr.Item == item })
}

// unlessOverridden is the type a derived name is built on, or nil when the item's own override replaces the whole name (CODEGEN.md §3.5).
func unlessOverridden(own NameOptions, from Type) any {
	if own.Name != "" {
		return nil
	}
	return from
}

// declareAll declares every name of the package scope in gen/go's section order (CODEGEN.md §2.7), and each struct's, container's, data's and parameter list's own scope.
func (pl *GoNamePlan) declareAll() {
	top := pl.scope(goScopePackage)
	if len(inputRecords(pl.p)) > 0 { // first, so a clash is reported at the user's item (CODEGEN.md §3.5, §5.12)
		pl.declare(top, loadInputs, pl.p.Name, nil)
	}
	for _, c := range pl.p.Consts {
		pl.declare(top, pl.ConstName(c), pl.p.Name+qnameSep+c.Name, c)
	}
	if pl.data != nil {
		pl.declareSchemas(top)
	}
	pl.declareEnums(top)
	pl.declareKindEnums(top)
	pl.declareIDEnums(top)
	pl.declareNestedIDs(top)
	for _, t := range pl.p.Types {
		pl.declareType(top, t)
	}
	pl.declareForeignRows(top)
	pl.declareHooks(top, pl.p, pl) // after the types and rows: their own names keep their findings (CODEGEN.md §2.7, §5.14)
	pl.declareEntryHooks(top)      // hook vs hook E8005 follows this order: type, entry, row (log-2026-10-06 "U1 re-review")
	pl.declareRowHooks(top)
	if pl.data != nil {
		pl.declareDataContainers(top)
		pl.declareLoaders(top)
		pl.declareSnapshot(top)
	} else {
		pl.declareContainers(top)
		pl.declareValues(top)
	}
	pl.declareFns(top)
	pl.declareDefines(top)
	pl.declareInputs(top)
	if pl.data != nil {
		pl.declareDecoders(top)
		pl.declareReaders(top)
		pl.declareResolvers(top)
		pl.declareLoads(top)
		pl.declareDataLocals()
	}
	pl.declareConformance(top)
	pl.declareImports(top)
}

// declareEnums declares each enum, its member constants, Parse<E>, <E>Members and <E>FromCode (CODEGEN.md §5.2).
func (pl *GoNamePlan) declareEnums(top *nameScope) {
	for _, t := range pl.p.Types {
		e, ok := t.(*Enum)
		if !ok {
			continue
		}
		name, origin := pl.TypeName(e), e.QName()
		pl.declare(top, name, origin, e)
		for _, m := range e.Members {
			pl.declareFrom(top, origin+qnameSep+m.Name, m, derivation{unlessOverridden(m.Go, e), func() string { return pl.MemberName(e, m) }})
		}
		pl.declare(top, pl.ParseName(name), origin, e)
		pl.declare(top, pl.MembersName(name), origin, e)
		if e.Codes != nil {
			pl.declare(top, pl.FromCodeName(name), origin, e)
		}
		pl.declareMethods(name, origin, e, pl.enumMethods(e))
	}
}

// declareKindEnums declares each variant's kind enum, its members and its Parse (CODEGEN.md §5.5).
func (pl *GoNamePlan) declareKindEnums(top *nameScope) {
	for _, t := range pl.p.Types {
		v, ok := t.(*Variant)
		if !ok {
			continue
		}
		kind, origin := pl.KindName(v), v.QName()
		pl.declare(top, kind, origin, v)
		for _, c := range v.Cases {
			pl.declareFrom(top, origin+qnameSep+c.Name, c, derivation{v, func() string { return pl.KindMemberName(v, c) }})
		}
		pl.declare(top, pl.ParseName(kind), origin, v)
		pl.declareMethods(kind, origin, v, goEnumMethods)
	}
}

// declareMethods declares the fixed methods of an enum, a kind enum or an id enum (CODEGEN.md §5.2, §5.3).
func (pl *GoNamePlan) declareMethods(goType, origin string, item any, methods []string) {
	sc := pl.scope(goType)
	for _, m := range methods {
		pl.declare(sc, m, origin, item)
	}
}

// declareIDEnums declares the id type of every public table value, emitted or not (decision 124): in baked mode an enum with its members, Parse and String, in data mode a string (CODEGEN.md §5.3).
func (pl *GoNamePlan) declareIDEnums(top *nameScope) {
	for _, v := range pl.p.Values {
		rec := tableRecord(v)
		if rec == nil {
			continue
		}
		name := pl.IDTypeName(rec)
		pl.declareFrom(top, v.Name, v, derivation{rec, func() string { return pl.IDTypeName(rec) }})
		if pl.data != nil {
			continue
		}
		for _, id := range v.IDs {
			pl.declareFrom(top, v.Name+qnameSep+id, v, derivation{rec, func() string { return pl.IDMemberName(rec, id) }})
		}
		pl.declareFrom(top, v.Name, v, derivation{rec, func() string { return pl.ParseName(pl.IDTypeName(rec)) }})
		pl.declareMethods(name, v.Name, v, goIDEnumMethods)
	}
}

// tableRecord is the element record of a table value, or nil.
func tableRecord(v *Value) *Record {
	if v.Type.Kind != types.Table || v.Type.Elem == nil {
		return nil
	}
	rec, _ := v.Type.Elem.Named.(*Record)
	return rec
}

// declareType declares a record, a variant and its case types, or a dependent type and its branch enum, with their own member scopes (CODEGEN.md §5.4–§5.6).
func (pl *GoNamePlan) declareType(top *nameScope, t Type) {
	switch x := t.(type) {
	case *Record:
		pl.declare(top, pl.TypeName(x), x.QName(), x)
		pl.declareBody(x.QName(), goStruct{x, func() string { return pl.TypeName(x) }}, x.Fields, x.Methods)
	case *Variant:
		pl.declareVariant(top, x)
	case *Dependent:
		pl.declareDependent(top, x)
	}
}

// declareVariant declares a variant, its selectors (kind, value, Kind, As<Case>) and each case with fields as a type of its own (CODEGEN.md §5.5).
func (pl *GoNamePlan) declareVariant(top *nameScope, v *Variant) {
	origin := v.QName()
	pl.declare(top, pl.TypeName(v), origin, v)
	sc := pl.scope(pl.TypeName(v))
	for _, n := range goVariantMembers {
		pl.declare(sc, n, origin, v)
	}
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			pl.declare(sc, pl.AsName(c), origin+qnameSep+c.Name, c)
		}
	}
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			st := goStruct{c, func() string { return pl.CaseName(v, c) }}
			pl.declareFrom(top, origin+qnameSep+c.Name, c, derivation{unlessOverridden(c.Go, v), st.name})
			pl.declareBody(origin+qnameSep+c.Name, st, c.Fields, c.Methods)
		}
	}
}

// bodyMember is one storage member or method of a struct, with what it is generated for.
type bodyMember struct {
	name, origin string
	item         any
}

// bodyNames are a struct's names in gen/go's order: the slots' members, the tables' members, then the slots' getters, the table-read methods and the translated ones (CODEGEN.md §5.4, §5.10).
type bodyNames struct {
	stores, tables, getters, finite []bodyMember
}

// goStruct is a record or case and how its Go type is named, which a translated method's pure function and test are built on (decision 213); zero at package level.
type goStruct struct {
	self any
	name func() string
}

// goName is the struct's Go type name, "" at package level.
func (st goStruct) goName() string {
	if st.name == nil {
		return ""
	}
	return st.name()
}

// declareBody declares the storage and methods of the struct st, one Go selector namespace (decision 182), getters first so a cause is reported at its getter (decision 203); a table entry has ID and Retired, id and retired first (CODEGEN.md §5.3).
func (pl *GoNamePlan) declareBody(owner string, st goStruct, fields []*Field, fns []*ExportFn) {
	var b bodyNames
	if rec, ok := st.self.(*Record); ok && pl.isTableRecord(rec) {
		b.stores = append(b.stores, bodyMember{GoIDStore, owner, rec}, bodyMember{GoRetiredStore, owner, rec})
		b.getters = append(b.getters, bodyMember{GoID, owner, rec}, bodyMember{GoRetired, owner, rec})
	}
	pl.addFields(&b, owner, fields)
	for _, fn := range fns {
		origin := owner + qnameSep + fn.Name
		switch fn.Kind {
		case FnPrecomputed:
			b.addSlot(origin, fn, pl.MethodSlot(fn))
		case FnLookup:
			pl.addFinite(&b, origin, fn)
		case FnTranslated:
			b.finite = append(b.finite, bodyMember{goExported(fn.Go, fn.Name), origin, fn})
			pl.declarePure(pl.scopes[0], origin, fn, st)
		}
	}
	sc := pl.scope(st.goName())
	for _, list := range [][]bodyMember{b.getters, b.finite, b.stores, b.tables} {
		for _, m := range list {
			pl.declare(sc, m.name, m.origin, m.item)
		}
	}
}

// addFields adds each field's storage and getters: none for a Never? field (CODEGEN.md §4.4), only the getter for an input field, which reads the package's slot (§5.12).
func (pl *GoNamePlan) addFields(b *bodyNames, owner string, fields []*Field) {
	for _, f := range fields {
		switch {
		case f.Optional && f.Type.Kind == types.Never:
		case f.Input != nil:
			b.getters = append(b.getters, bodyMember{pl.Slot(f).Getter, owner + qnameSep + f.Name, f})
		default:
			b.addSlot(owner+qnameSep+f.Name, f, pl.Slot(f))
		}
	}
}

// addFinite adds a lookup method's table, and in data mode the table of a resolved result's keys, then its entry getter when it has one and a ref result's key getter <Name>ID(s) (CODEGEN.md §5.8, §5.10; log-2026-09-24 "gen/go review calls").
func (pl *GoNamePlan) addFinite(b *bodyNames, origin string, fn *ExportFn) {
	f := pl.Finite(fn)
	b.tables = append(b.tables, bodyMember{f.Store, origin, fn})
	if pl.data != nil && f.Result.Resolved {
		b.tables = append(b.tables, bodyMember{f.Result.KeyStore, origin, fn})
	}
	if f.Result.Main {
		b.finite = append(b.finite, bodyMember{f.Name, origin, fn})
	}
	if f.Result.Ref != nil {
		b.finite = append(b.finite, bodyMember{f.Result.KeyGetter, origin, fn})
	}
	pl.declareParams(goSelf, origin, fn)
}

// addSlot adds a slot's members and getters, those it has.
func (b *bodyNames) addSlot(origin string, item any, s GoSlot) {
	stores, getters := s.members()
	b.stores = append(b.stores, asMembers(origin, item, stores)...)
	b.getters = append(b.getters, asMembers(origin, item, getters)...)
}

// asMembers are names generated for item.
func asMembers(origin string, item any, names []string) []bodyMember {
	out := make([]bodyMember, len(names))
	for i, n := range names {
		out[i] = bodyMember{n, origin, item}
	}
	return out
}

// isTableRecord reports a record some public table value or table field holds: its entries have an id (CODEGEN.md §4.2, §5.3).
func (pl *GoNamePlan) isTableRecord(rec *Record) bool {
	return pl.isTableValueRecord(rec) || pl.NestedRow(rec)
}

// declareParams declares a table-read function's receiver or table, its parameters' locals, escaped like the imports (decision 182), and their index locals (decision 122).
func (pl *GoNamePlan) declareParams(root, origin string, fn *ExportFn) {
	sc := pl.scope(origin + goParamsSuffix)
	pl.declare(sc, root, origin, fn)
	for _, p := range fn.Params {
		name := goStorageName(p.Name)
		local, ok := pl.local(name)
		if !ok {
			pl.problems = append(pl.problems, GoNameProblem{Kind: GoCollision, Scope: sc.what, Name: local, First: pl.imports[local], Origin: origin + qnameSep + p.Name, Item: p})
		}
		pl.declare(sc, local, origin+qnameSep+p.Name, p)
		if index := indexLocal(local, p.Type); index != "" {
			pl.declare(sc, index, origin+qnameSep+p.Name, p)
		}
	}
}

// warnMacro is W8006 for a declared name that is a macro of common platform headers, once per name and origin (CODEGEN.md §3.5).
func (n *namer) warnMacro(sc *nameScope, name, origin string, item any) {
	pair := originPair{name, origin}
	if !n.macros[name] || n.warned[pair] {
		return
	}
	n.warned[pair] = true
	n.warnings = append(n.warnings, GoNameProblem{Kind: goMacroName, Scope: sc.what, Name: name, Origin: origin, Item: item})
}
