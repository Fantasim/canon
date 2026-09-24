package ir

import (
	"github.com/fantasim/canonlang/internal/types"
)

// nameScope is one namespace of generated code and what declared each of its names (§3.5).
type nameScope struct {
	what  string
	names map[string]string
}

// namer is a name plan's scopes and problems; ident tells an identifier of the target.
type namer struct {
	scopes   []*nameScope
	problems []GoNameProblem
	reported map[originPair]bool // the pairs of origins already reported colliding: one E8005 per cause (decision 203)
	ident    func(string) bool
}

func newNamer(ident func(string) bool) namer {
	return namer{reported: map[originPair]bool{}, ident: ident}
}

// scope opens a new scope of the plan.
func (n *namer) scope(what string) *nameScope {
	sc := &nameScope{what: what, names: map[string]string{}}
	n.scopes = append(n.scopes, sc)
	return sc
}

// declare adds name to sc for origin; a name that is no identifier of the target (decision 202), or that sc already holds, is a problem. Two origins collide once in the whole plan, however many of their names meet, in any scope (decision 203).
func (n *namer) declare(sc *nameScope, name, origin string, item any) {
	if !n.ident(name) {
		n.problems = append(n.problems, GoNameProblem{Kind: GoNotIdentifier, Scope: sc.what, Name: name, Origin: origin, Item: item})
		return
	}
	if first, ok := sc.names[name]; ok {
		if pair := (originPair{first, origin}); !n.reported[pair] {
			n.reported[pair] = true
			n.problems = append(n.problems, GoNameProblem{Kind: GoCollision, Scope: sc.what, Name: name, First: first, Origin: origin, Item: item})
		}
		return
	}
	sc.names[name] = origin
}

// declareAll declares every name of the package scope in gen/go's section order (CODEGEN.md §2.7), and each struct's, container's, data's and parameter list's own scope.
func (pl *GoNamePlan) declareAll() {
	top := pl.scope(goScopePackage)
	for _, c := range pl.p.Consts {
		pl.declare(top, pl.ConstName(c), c.Name, c)
	}
	if pl.data != nil {
		pl.declareSchemas(top)
	}
	pl.declareEnums(top)
	pl.declareKindEnums(top)
	pl.declareIDEnums(top)
	for _, t := range pl.p.Types {
		pl.declareType(top, t)
	}
	if pl.data != nil {
		pl.declareDataContainers(top)
		pl.declareLoaders(top)
		pl.declareSnapshot(top)
	} else {
		pl.declareContainers(top)
		pl.declareValues(top)
	}
	pl.declareFns(top)
	if pl.data != nil {
		pl.declareDecoders(top)
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
			pl.declare(top, pl.MemberName(e, m), origin+qnameSep+m.Name, m)
		}
		pl.declare(top, pl.ParseName(name), origin, e)
		pl.declare(top, pl.MembersName(name), origin, e)
		methods := goEnumMethods
		if e.Codes != nil {
			pl.declare(top, pl.FromCodeName(name), origin, e)
			methods = goCodesEnumMethods
		}
		pl.declareMethods(name, origin, e, methods)
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
			pl.declare(top, pl.KindMemberName(v, c), origin+qnameSep+c.Name, c)
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
		pl.declare(top, name, v.Name, v)
		if pl.data != nil {
			continue
		}
		for _, id := range v.IDs {
			pl.declare(top, pl.IDMemberName(rec, id), v.Name+qnameSep+id, v)
		}
		pl.declare(top, pl.ParseName(name), v.Name, v)
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

// declareType declares a record, or a variant and its case types, with their own member scopes (CODEGEN.md §5.4, §5.5); a dependent type, which baked gen/go refuses (decision 124), declares nothing yet.
func (pl *GoNamePlan) declareType(top *nameScope, t Type) {
	switch x := t.(type) {
	case *Record:
		pl.declare(top, pl.TypeName(x), x.QName(), x)
		pl.declareBody(pl.TypeName(x), x.QName(), x, x.Fields, x.Methods)
	case *Variant:
		pl.declareVariant(top, x)
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
			pl.declare(top, pl.CaseName(v, c), origin+qnameSep+c.Name, c)
			pl.declareBody(pl.CaseName(v, c), origin+qnameSep+c.Name, nil, c.Fields, c.Methods)
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

// declareBody declares the storage and methods of the struct goName, one Go selector namespace (decision 182), getters first so a cause is reported at its getter (decision 203). rec is the record, nil for a case; a table entry has ID and Retired, id and retired first (CODEGEN.md §5.3).
func (pl *GoNamePlan) declareBody(goName, owner string, rec *Record, fields []*Field, fns []*ExportFn) {
	var b bodyNames
	if rec != nil && pl.isTableRecord(rec) {
		b.stores = append(b.stores, bodyMember{GoIDStore, owner, rec}, bodyMember{GoRetiredStore, owner, rec})
		b.getters = append(b.getters, bodyMember{GoID, owner, rec}, bodyMember{GoRetired, owner, rec})
	}
	for _, f := range fields {
		if f.Optional && f.Type.Kind == types.Never {
			continue // CODEGEN.md §4.4: a Never? field is not emitted at all
		}
		b.addSlot(owner+qnameSep+f.Name, f, pl.Slot(f))
	}
	for _, fn := range fns {
		origin := owner + qnameSep + fn.Name
		switch fn.Kind {
		case FnPrecomputed:
			b.addSlot(origin, fn, pl.MethodSlot(fn))
		case FnLookup:
			pl.addFinite(&b, origin, fn)
		case FnTranslated:
			b.finite = append(b.finite, bodyMember{goExported(fn.Go, fn.Name), origin, fn})
			pl.declarePure(pl.scopes[0], goName, origin, fn)
		}
	}
	sc := pl.scope(goName)
	for _, list := range [][]bodyMember{b.getters, b.finite, b.stores, b.tables} {
		for _, m := range list {
			pl.declare(sc, m.name, m.origin, m.item)
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

// isTableRecord reports a record some public table value holds: its entries have an id (CODEGEN.md §5.3).
func (pl *GoNamePlan) isTableRecord(rec *Record) bool {
	for _, v := range pl.p.Values {
		if tableRecord(v) == rec {
			return true
		}
	}
	return false
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
