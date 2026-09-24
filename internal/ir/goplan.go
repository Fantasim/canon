package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// GoNamePlan is every Go name baked Go declares for a package and its go emit, per scope (CODEGEN.md §3.3–§3.5): stage E reports E8005 and E8011 from its problems and gen/go writes every name from its lookups (decision 194); a lookup also names another package's type, member or id.
type GoNamePlan struct {
	p        *Package
	e        *Emit
	emitted  []*Value
	byValue  map[string]*Value
	pkgFns   map[*ExportFn]bool
	imports  map[string]string // the Go package name of each imported Canon package's go emit → its import path
	scopes   []*goScope
	problems []GoNameProblem
	reported map[originPair]bool // the pairs of origins already reported colliding: one E8005 per cause (decision 203)
}

// GoNameProblem is a name gen/go cannot declare (CODEGEN.md §3.5, decision 182). Item is the IR node Origin names (a Type, *EnumMember, *Case, *Field, *ExportFn, *Param, *Const or *Value), nil for the package or an import.
type GoNameProblem struct {
	Kind          GoProblemKind
	Scope, Name   string
	First, Origin string // First: what declared Name before Origin, for a collision
	Item          any
}

// originPair is two origins whose names collide, the first declared first.
type originPair struct{ first, second string }

// GoProblemKind is what is wrong with a name of the plan.
type GoProblemKind uint8

// GoSlot is how baked Go stores and reads a field, value or stored result (CODEGEN.md §4.3, §5.8): Main holds the value or a ref's resolved entry, Key a ref's key (else read off the entry), OK an optional nil cannot mark.
type GoSlot struct {
	T                        TypeRef // without the outer `?`
	Optional                 bool
	Ref                      *RefTarget // the target of `ref T` or `[ref T]`
	List, Resolved           bool
	Main, Key, OK            bool
	Getter, KeyGetter        string
	Store, KeyStore, OKStore string
}

// GoFinite is a stored export fn read from a table (CODEGEN.md §5.10): its function, its table's member or variable, its parameters' locals, the index local of each parameter that needs one ("" when the parameter indexes itself) and how a cell of its result is held.
type GoFinite struct {
	Name, Store string
	Params      []string
	Indexes     []string
	Result      GoSlot
}

// GoData is the baked data (CODEGEN.md §6.2): its struct, sync.OnceValue and builder, and its local d (decision 182).
type GoData struct {
	Type, Values, Build, Local string
}

// PlanGoNames is the name plan of p's go emit e, with every problem found.
func PlanGoNames(p *Package, e *Emit) *GoNamePlan {
	pl := &GoNamePlan{p: p, e: e, byValue: map[string]*Value{}, pkgFns: map[*ExportFn]bool{}, imports: map[string]string{}, reported: map[originPair]bool{}}
	for _, fn := range p.Fns {
		pl.pkgFns[fn] = true
	}
	for _, ref := range p.Imports {
		for _, imp := range ref.Emits {
			if imp.Target == TargetGo {
				pl.imports[imp.GoPackage] = imp.GoImport
			}
		}
	}
	for _, v := range p.Values {
		if len(e.Values) == 0 || slices.Contains(e.Values, v.Name) {
			pl.emitted = append(pl.emitted, v)
			pl.byValue[v.Name] = v
		}
	}
	pl.problems = goOverrideProblems(p)
	pl.declareAll()
	return pl
}

// Problems are the plan's problems: overrides first, then names in generation order.
func (pl *GoNamePlan) Problems() []GoNameProblem { return pl.problems }

// Emitted are the values the emit selects, in declaration order (CODEGEN.md §2.2).
func (pl *GoNamePlan) Emitted() []*Value { return pl.emitted }

// TypeName is a record's, enum's, variant's or dependent type's Go name in its own package: its first letter upper-cased, or its override; "" for another Type.
func (pl *GoNamePlan) TypeName(t Type) string {
	switch x := t.(type) {
	case *Record:
		return goTypeName(x.Go, x.Name)
	case *Enum:
		return goTypeName(x.Go, x.Name)
	case *Variant:
		return goTypeName(x.Go, x.Name)
	case *Dependent:
		return goTypeName(x.Go, x.Name)
	}
	return ""
}

// MemberName is an enum member's constant: the enum + UpperCamel(m), or the whole override (CODEGEN.md §3.3).
func (pl *GoNamePlan) MemberName(e *Enum, m *EnumMember) string {
	if m.Go.Name != "" {
		return m.Go.Name
	}
	return pl.TypeName(e) + goUpperCamel(m.Name)
}

// KindName is a variant's kind enum, TKind (CODEGEN.md §5.5).
func (pl *GoNamePlan) KindName(v Type) string { return pl.TypeName(v) + GoKind }

// KindMemberName is a case's member of its kind enum: TKind + the case's exported name (decision 193).
func (pl *GoNamePlan) KindMemberName(v *Variant, c *Case) string {
	return pl.KindName(v) + goExported(c.Go, c.Name)
}

// CaseName is a case's type: T + UpperCamel(c), or the whole override (CODEGEN.md §3.3, decision 193).
func (pl *GoNamePlan) CaseName(v *Variant, c *Case) string {
	if c.Go.Name != "" {
		return c.Go.Name
	}
	return pl.TypeName(v) + goUpperCamel(c.Name)
}

// AsName is the variant's accessor of a case with fields, As + the case's exported name (§5.5).
func (pl *GoNamePlan) AsName(c *Case) string { return goAsPrefix + goExported(c.Go, c.Name) }

// IDTypeName is the id enum of a table of elem, <Element>ID (CODEGEN.md §5.3, decision 193).
func (pl *GoNamePlan) IDTypeName(elem Type) string { return pl.TypeName(elem) + GoID }

// IDMemberName is the id constant of one table key, <Element>ID + UpperCamel(key).
func (pl *GoNamePlan) IDMemberName(elem Type, key string) string {
	return pl.IDTypeName(elem) + goUpperCamel(key)
}

// ParseName is Parse<E> of the enum, kind enum or id enum named enum (CODEGEN.md §5.2).
func (pl *GoNamePlan) ParseName(enum string) string { return goParsePrefix + enum }

// MembersName is <E>Members of an enum (CODEGEN.md §5.2).
func (pl *GoNamePlan) MembersName(enum string) string { return enum + goMembersSuffix }

// FromCodeName is <E>FromCode of an enum with @codes (CODEGEN.md §5.2).
func (pl *GoNamePlan) FromCodeName(enum string) string { return enum + goFromCodeSuffix }

// ConstName is a constant's Go name (CODEGEN.md §3.3).
func (pl *GoNamePlan) ConstName(c *Const) string { return goExported(c.Go, c.Name) }

// ContainerName is the class of a table or keyed-list value, UpperCamel(v) (CODEGEN.md §5.9).
func (pl *GoNamePlan) ContainerName(v *Value) string { return goUpperCamel(v.Name) }

// AccessorName is Get<V>, or the value's whole override (CODEGEN.md §3.3, §5.9).
func (pl *GoNamePlan) AccessorName(v *Value) string {
	if v.Go.Name != "" {
		return v.Go.Name
	}
	return GoGet + goUpperCamel(v.Name)
}

// ValueStore is the member of the baked data that holds value v (CODEGEN.md §6.2), from its effective name (decision 203).
func (pl *GoNamePlan) ValueStore(v *Value) string { return goEffectiveStore(v.Go, v.Name) }

// FindByName is a table container's FindBy<F> of a @stable field (CODEGEN.md §5.9).
func (pl *GoNamePlan) FindByName(f *Field) string { return goFindByPrefix + goExported(f.Go, f.Name) }

// FindByIndex is the package variable of FindBy<F>'s index (decision 193), from the effective names of the value and the field (decision 203).
func (pl *GoNamePlan) FindByIndex(v *Value, f *Field) string {
	return pl.ValueStore(v) + goUpperCamel(goEffective(f.Go, f.Name)) + goIndexSuffix
}

// Data is the baked data's names (CODEGEN.md §6.2): <last>Data, <last>Values, build<Last>.
func (pl *GoNamePlan) Data() GoData {
	segs := strings.Split(pl.p.Name, qnameSep)
	last := goLowerCamel(segs[len(segs)-1])
	local, _ := pl.local(goDataLocal)
	build := goBuildPrefix + goTypeName(NameOptions{}, last)
	return GoData{Type: last + goDataSuffix, Values: last + goValuesSuffix, Build: build, Local: local}
}

// Slot is a record's or case's field as a slot: its getter is the field's exported name.
func (pl *GoNamePlan) Slot(f *Field) GoSlot {
	return pl.slot(f.Type, f.Optional, goExported(f.Go, f.Name), goEffectiveStore(f.Go, f.Name))
}

// ValueSlot is a value that is not a container, read like a field of its type (§5.9).
func (pl *GoNamePlan) ValueSlot(v *Value) GoSlot {
	t, opt := goUnwrap(v.Type)
	return pl.slot(t, opt, pl.AccessorName(v), pl.ValueStore(v))
}

// MethodSlot is a precomputed export method, a getter named after the fn (CODEGEN.md §5.4).
func (pl *GoNamePlan) MethodSlot(fn *ExportFn) GoSlot {
	t, opt := goUnwrap(fn.Result)
	return pl.slot(t, opt, goExported(fn.Go, fn.Name), goEffectiveStore(fn.Go, fn.Name))
}

// Finite is a stored fn read from a table: a method's is a member of its struct, a package fn's the variable <fn>Table (CODEGEN.md §5.10, decision 183).
func (pl *GoNamePlan) Finite(fn *ExportFn) GoFinite {
	f := GoFinite{Name: goExported(fn.Go, fn.Name), Store: goEffectiveStore(fn.Go, fn.Name)}
	if pl.pkgFns[fn] {
		f.Store = goLowerCamel(goEffective(fn.Go, fn.Name)) + goTableSuffix
	}
	for _, p := range fn.Params {
		local, _ := pl.local(goStorageName(p.Name))
		f.Params = append(f.Params, local)
		f.Indexes = append(f.Indexes, indexLocal(local, p.Type))
	}
	t, opt := goUnwrap(fn.Result)
	f.Result = pl.slot(t, opt, f.Name, f.Store)
	return f
}

// indexLocal is the local a table read computes its index into (decision 122): local_i for a Bool or an enum with @codes, "" for a parameter that indexes itself.
func indexLocal(local string, t TypeRef) string {
	if e, ok := t.Named.(*Enum); t.Kind == types.Bool || ok && t.Kind == types.Enum && e.Codes != nil {
		return local + goIndexLocalSuffix
	}
	return ""
}

// goUnwrap splits a type T? into T and true.
func goUnwrap(t TypeRef) (TypeRef, bool) {
	if t.Kind == types.Optional && t.Elem != nil {
		return *t.Elem, true
	}
	return t, false
}

// slot lays out a slot of type t (CODEGEN.md §4.3, §5.8): a resolved ref stores its entry, an unresolved one its key, a resolved list both; an optional value nil cannot mark gets ok.
func (pl *GoNamePlan) slot(t TypeRef, opt bool, getter, store string) GoSlot {
	s := GoSlot{T: t, Optional: opt}
	switch {
	case t.Kind == types.Ref:
		s.Ref = t.Ref
	case t.Kind == types.List && t.KeyedBy == nil && t.Elem != nil && t.Elem.Kind == types.Ref:
		s.Ref, s.List = t.Elem.Ref, true
	}
	s.Resolved = s.Ref != nil && pl.resolvable(s.Ref)
	s.Main = s.Ref == nil || s.Resolved
	s.Key = s.Ref != nil && (!s.Resolved || s.List)
	pointer := s.Ref == nil && goPointerKinds[t.Kind] || s.Ref != nil && !s.List
	s.OK = opt && (!s.Main || s.Key || !pointer)
	s.Getter, s.Store, s.OKStore = getter, store, store+goOKStoreSuffix
	s.KeyGetter, s.KeyStore = getter+GoID, store+goKeyStoreSuffix
	if s.List {
		s.KeyGetter, s.KeyStore = s.KeyGetter+goPluralSuffix, store+goKeysStoreSuffix
	}
	return s
}

// members are the slot's storage members, then its getters: main, key, ok (CODEGEN.md §4.3, §5.8).
func (s GoSlot) members() (stores, getters []string) {
	if s.Main {
		stores, getters = append(stores, s.Store), append(getters, s.Getter)
	}
	if s.Key {
		stores = append(stores, s.KeyStore)
	}
	if s.OK {
		stores = append(stores, s.OKStore)
	}
	if s.Ref != nil {
		getters = append(getters, s.KeyGetter)
	}
	return stores, getters
}

// resolvable reports a ref into an emitted value of this package: its getter returns the entry (CODEGEN.md §5.8).
func (pl *GoNamePlan) resolvable(r *RefTarget) bool {
	return r.Coll == types.CollLet && !r.Local && r.Pkg == pl.p.Name && pl.byValue[r.Value] != nil
}

// local escapes a parameter or local named like an imported Canon package (CODEGEN.md §3.4, decision 182); false when the escaped name is an import too.
func (pl *GoNamePlan) local(name string) (string, bool) {
	if _, clash := pl.imports[name]; !clash {
		return name, true
	}
	_, clash := pl.imports[name+underscore]
	return name + underscore, !clash
}
