package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// CppFindBy are a @stable field's container lookup FindBy<F> and its two sorted indexes, by<F>Keys_ and by<F>_ (CODEGEN.md §5.9).
type CppFindBy struct {
	Func, Keys, Index string
}

// CppFindTables are a baked container lookup's sorted keys and their rows' positions, detail::k<C><Lookup>Keys and k<C><Lookup>Rows of its .gen.cpp, <Lookup> being Find or FindBy<F> (CODEGEN.md §5.9, DECISIONS 432, 435).
type CppFindTables struct {
	Keys, Rows string
}

// CppVector are a translated fn's conformance vector struct <Owner><Fn>Vector and its table k<Owner><Fn> (CONFORMANCE.md §7.2).
type CppVector struct {
	Struct, Table string
}

// CppEnumHelpers are an enum's k<E>Members, k<E>LiveMembers (DECISIONS 395), <E>FromWire and, with @codes, <E>FromCode; a closed @codes enum's detail::k<E>Codes and detail::k<E>CodeOrdinals (not when its codes are dense) and detail::<E>Ordinal; the .gen.cpp's detail::k<E>Names, k<E>Wires (only when a wire differs from its name, always for an id enum) and k<E>ByWire and k<E>Retired (the retired flags in declaration order, IsRetired's table) of a closed enum, a kind enum and an id enum; an opened enum's detail::k<E>Wires, k<E>WireNames (only when a name differs from its wire), k<E>Retired (flags parallel to k<E>Wires) and, with @codes, k<E>WireCodes, k<E>Codes (ascending) and k<E>CodeAt (the place in k<E>Wires of each of those codes) (CODEGEN.md §5.2, DECISIONS 383, 395, 430).
type CppEnumHelpers struct {
	Members, LiveMembers         string
	FromWire, FromCode           string
	Codes, CodeOrdinals, Ordinal string
	Wires, WireNames, WireCodes  string
	Names, ByWire, Retired       string
	CodeAt                       string
}

// FindBy is the lookup and indexes of @stable field f (CODEGEN.md §5.9).
func (pl *CppNamePlan) FindBy(f *Field) CppFindBy {
	upper := cppUpperCamel(f.Name)
	by := cppByPrefix + upper
	return CppFindBy{Func: goFindByPrefix + upper, Keys: by + cppKeys + underscore, Index: by + underscore}
}

// FindTables are the tables of keyed list v's Find, which the .gen.cpp holds in an unnamed namespace of detail, local to it (CODEGEN.md §5.9).
func (pl *CppNamePlan) FindTables(v *Value) CppFindTables {
	return pl.lookupTables(pl.ContainerName(v), GoFind)
}

// FindByTables are the tables of FindBy<F> of @stable field f of container v.
func (pl *CppNamePlan) FindByTables(v *Value, f *Field) CppFindTables {
	return pl.lookupTables(pl.ContainerName(v), pl.FindBy(f).Func)
}

// lookupTables are the tables k<C><Lookup>Keys and k<C><Lookup>Rows of lookup (Find or FindBy<F>) of container.
func (pl *CppNamePlan) lookupTables(container, lookup string) CppFindTables {
	base := cppConstPrefix + container + lookup
	return CppFindTables{Keys: base + cppKeys, Rows: base + cppUpperCamel(GoRows)}
}

// declareFindTables declares, in detail, the tables of each baked container's Find (a keyed list's) and FindBy<F>, which its .gen.cpp holds in an unnamed namespace local to it: k<C>FindKeys and k<C>FindRows, k<C>FindBy<F>Keys and k<C>FindBy<F>Rows (CODEGEN.md §5.9, DECISIONS 432, 435).
func (pl *CppNamePlan) declareFindTables(detail *nameScope) {
	for _, v := range pl.values {
		rec := containerRecord(v)
		if !IsContainer(v) || rec == nil {
			continue
		}
		if v.Type.KeyedBy != nil {
			pl.declareFindPair(detail, pl.FindTables(v), pl.valueOrigin(v), v)
		}
		for _, f := range rec.Fields {
			if findsBy(f) {
				pl.declareFindPair(detail, pl.FindByTables(v, f), rec.QName()+qnameSep+f.Name, f)
			}
		}
	}
}

// declareFindPair declares a lookup's two tables, built on Canon names @cpp(name:) does not change (E8005 foreignDerived).
func (pl *CppNamePlan) declareFindPair(detail *nameScope, t CppFindTables, origin string, item any) {
	for _, n := range []string{t.Keys, t.Rows} {
		pl.noteDeclared(detail, n, foreignNote{derived: true}, func() { pl.localInner(detail, n, origin, item, meetsLocal) })
	}
}

// SnapshotGetter is the snapshot's getter of @reload value v, Get + UpperCamel(v) or its @cpp(name:) (CODEGEN.md §3.3, §5.11).
func (pl *CppNamePlan) SnapshotGetter(v *Value) string {
	return cppOverride(v.Cpp, GoGet+cppUpperCamel(v.Name))
}

// StoredGetter is a stored method's getter, UpperCamel(fn) or its @cpp(name:), then Key or Keys for a ref result; the resolved getter is the name without them (CODEGEN.md §3.3, §5.8).
func (pl *CppNamePlan) StoredGetter(fn *ExportFn) (getter, resolved string) {
	t, _ := goUnwrap(fn.Result)
	resolved = pl.FnName(fn)
	return resolved + cppKeySuffix(t), resolved
}

// RefMember is the member holding a resolved ref's entry, f_ref_, beside the key's f_ (CODEGEN.md §5.8, §7.2).
func (pl *CppNamePlan) RefMember(canon string) string { return pl.Member(canon) + cppRefSuffix }

// Vector is fn's vector struct and table; owner is its class's name, "" for a package fn (CONFORMANCE.md §7.2).
func (pl *CppNamePlan) Vector(owner string, fn *ExportFn) CppVector {
	base := owner + cppUpperCamel(fn.Name)
	return CppVector{Struct: base + cppVectorSuffix, Table: cppConstPrefix + base}
}

// EnumHelpers are the helpers of the enum or kind enum named enum (CODEGEN.md §5.2, §5.5).
func (pl *CppNamePlan) EnumHelpers(enum string) CppEnumHelpers {
	return CppEnumHelpers{
		Members: cppConstPrefix + enum + goMembersSuffix, LiveMembers: cppConstPrefix + enum + liveMembersSuffix, FromWire: enum + cppFromWire, FromCode: enum + goFromCodeSuffix,
		Codes: cppConstPrefix + enum + cppCodes, CodeOrdinals: cppConstPrefix + enum + cppCodeOrdinals, Ordinal: enum + cppOrdinal,
		Wires: cppConstPrefix + enum + cppWires, WireNames: cppConstPrefix + enum + cppWireNames, WireCodes: cppConstPrefix + enum + cppWireCodes,
		Names: cppConstPrefix + enum + cppNames, ByWire: cppConstPrefix + enum + cppByWire, Retired: cppConstPrefix + enum + GoRetired,
		CodeAt: cppConstPrefix + enum + GoCode + GoAt,
	}
}

// declareWireTables declares, in detail, each opened enum's sorted wires and their parallel retired flags and codes, its sorted codes with their places in the wires and, only when a name differs from its wire, names, which the .gen.cpp holds in an unnamed namespace, local to it as the conformance file's helpers are (CODEGEN.md §5.2, DECISIONS 383, 430).
func (pl *CppNamePlan) declareWireTables(detail *nameScope) {
	for _, e := range pl.OpenEnums() {
		h := pl.EnumHelpers(pl.TypeName(e))
		names := []string{h.Wires, h.Retired}
		if !EnumWiresAreNames(e) {
			names = append(names, h.WireNames)
		}
		if e.Codes != nil {
			names = append(names, h.WireCodes, h.Codes, h.CodeAt)
		}
		for _, n := range names {
			pl.localInner(detail, n, e.QName(), e, meetsLocal)
		}
	}
}

// declareOrdinals declares, in detail, each closed @codes enum's <E>Ordinal, which the header defines, and, unless its codes are dense, its k<E>Codes and k<E>CodeOrdinals (CODEGEN.md §5.2, §7.1; DECISIONS 383, 430).
func (pl *CppNamePlan) declareOrdinals(detail *nameScope) {
	for _, t := range pl.p.Types {
		if e, ok := t.(*Enum); ok && e.Codes != nil && !OpenEnum(pl.p, pl.e, e) {
			h := pl.EnumHelpers(pl.TypeName(e))
			names := []string{h.Ordinal}
			if _, dense := DenseCodes(e); !dense {
				names = append(names, h.Codes, h.CodeOrdinals)
			}
			for _, n := range names {
				pl.shareInner(detail, n, e.QName(), e)
			}
		}
	}
}

// declareEnumTables declares, in detail, the tables the .gen.cpp holds before the helpers of each closed enum, kind enum and table id enum, in an unnamed namespace local to it: k<E>Names (not for an id enum), k<E>Wires (only when a wire differs from its name, always for an id enum), k<E>ByWire and k<E>Retired (CODEGEN.md §5.2, §5.3; DECISIONS 395, 430). An id enum's finding names its record, as the enum's own does (declareIDEnum).
func (pl *CppNamePlan) declareEnumTables(detail *nameScope) {
	for _, t := range pl.p.Types {
		switch x := t.(type) {
		case *Enum:
			if !OpenEnum(pl.p, pl.e, x) {
				pl.declareTables(detail, pl.TypeName(x), x.QName(), x, EnumWiresAreNames(x))
			}
		case *Variant:
			pl.declareTables(detail, pl.KindName(x), x.QName(), x, CaseWiresAreNames(x))
		}
	}
	if !pl.baked() {
		return
	}
	for _, v := range pl.p.Values {
		if rec := tableRecord(v); rec != nil {
			h, byRecord := pl.EnumHelpers(pl.IDName(rec)), foreignNote{origin: rec.QName()}
			for _, n := range []string{h.Wires, h.ByWire, h.Retired} {
				pl.noteDeclared(detail, n, byRecord, func() { pl.localInner(detail, n, pl.valueOrigin(v), v, meetsLocal) })
			}
		}
	}
}

// declareTables declares enum name's k<E>Names, its k<E>Wires unless the wires are the names, its k<E>ByWire and its k<E>Retired.
func (pl *CppNamePlan) declareTables(detail *nameScope, name, origin string, item any, wiresAreNames bool) {
	h := pl.EnumHelpers(name)
	names := []string{h.Names, h.ByWire, h.Retired}
	if !wiresAreNames {
		names = append(names, h.Wires)
	}
	for _, n := range names {
		pl.localInner(detail, n, origin, item, meetsLocal)
	}
}

// AccessName is detail's access struct, <P>Access (CODEGEN.md §3.3, §7.6).
func (pl *CppNamePlan) AccessName() string { return pl.Upper() + cppAccessSuffix }

// AccessLoader is the access struct's loader of value v, Load<V> (CODEGEN.md §7.6).
func (pl *CppNamePlan) AccessLoader(v *Value) string { return CppLoad + cppUpperCamel(v.Name) }

// AccessSnapshotLoader is the access struct's LoadSnapshot, which the snapshot's Load calls (CODEGEN.md §5.11, §7.6).
func (pl *CppNamePlan) AccessSnapshotLoader() string { return CppLoad + goSnapshotSuffix }

// SnapshotName is <P>Snapshot (CODEGEN.md §3.3, §5.11).
func (pl *CppNamePlan) SnapshotName() string { return pl.Upper() + goSnapshotSuffix }

// StoreName is <P>Store (CODEGEN.md §3.3, §5.11).
func (pl *CppNamePlan) StoreName() string { return pl.Upper() + goStoreSuffix }

// RunConformanceName is conformance's entry point, Run<P>Conformance (CODEGEN.md §3.3).
func (pl *CppNamePlan) RunConformanceName() string {
	return cppRunPrefix + pl.Upper() + goConformanceSuffix
}

// CppDependent are a dependent type's C++ names (CODEGEN.md §3.3, §5.6): its class, its branch enum TBranch, GetBranch, the class's std::variant, detail's Decode<Alias>, and each branch's names in arm order.
type CppDependent struct {
	Class, Branch, GetBranch, Value, Decode string
	DefineValue                             string // the member holding a define branch's value, "" without one (DECISIONS 298)
	Branches                                []CppBranch
}

// CppBranch is one branch's TBranch enumerator, its As<Branch> and, for a ref into a load.defines table, its As<Branch>Value ("" else).
type CppBranch struct {
	Enumerator, As, AsValue string
}

// CppInputs are runtime inputs' names (CODEGEN.md §5.12, §7.7): LoadInputs in the namespace, detail's <P>Inputs, which holds one namespace of slots per class, <P>InputsLoaded, the flag every input getter checks, and the helpers the package's inputs use, in the order gen/cpp writes them.
type CppInputs struct {
	Func, Namespace, Loaded string
	Helpers                 []string
}

// Dependent is d's C++ names, of this package or another: a @cpp(name:) override replaces T in every name built on it (CODEGEN.md §3.5).
func (pl *CppNamePlan) Dependent(d *Dependent) CppDependent {
	name := pl.TypeName(d)
	out := CppDependent{Class: name, Branch: name + branchWord, GetBranch: GoGet + branchWord, Value: CppVariantMember, Decode: CppDecode + name}
	for _, b := range d.Branches {
		br := CppBranch{Enumerator: cppVerbatim(b.Name), As: cppAsPrefix + cppUpperCamel(b.Name)}
		if DefinesRef(b.Type) {
			br.AsValue = br.As + asValueSuffix
			out.DefineValue = cppDefineValue
		}
		out.Branches = append(out.Branches, br)
	}
	return out
}

// Inputs are the runtime inputs' names; the plan declares them only when a record of the package has an input field. MatchPattern comes last, for a patterned input.
func (pl *CppNamePlan) Inputs() CppInputs {
	used := map[types.Kind]bool{}
	patterned := false
	for _, rec := range inputRecords(pl.p) {
		for _, f := range inputFields(rec) {
			used[f.Type.Kind] = true
			patterned = patterned || len(f.Patterns) > 0
		}
	}
	var helpers []string
	for _, h := range cppInputHelpers {
		if h.kinds == nil || slices.ContainsFunc(h.kinds, func(k types.Kind) bool { return used[k] }) {
			helpers = append(helpers, h.names...)
		}
	}
	if patterned {
		helpers = append(helpers, CppMatchPattern)
	}
	return CppInputs{Func: loadInputs, Namespace: pl.Upper() + cppInputsSuffix, Loaded: pl.Upper() + cppInputsLoaded, Helpers: helpers}
}

// InputParser is the §7.7 parser of an input of type t; "" for an enum, read by <E>FromWire.
func (pl *CppNamePlan) InputParser(t TypeRef) string {
	for _, h := range cppInputHelpers {
		if len(h.kinds) == 1 && h.kinds[0] == t.Kind {
			return h.names[len(h.names)-1]
		}
	}
	return ""
}

// declareInputNames declares LoadInputs and the helpers the package's inputs use in the namespace: the helpers are in the sources' anonymous namespace, which the namespace sees, so each meets only another package's helpers legally (CODEGEN.md §3.5, §7.7).
func (pl *CppNamePlan) declareInputNames() {
	if len(inputRecords(pl.p)) == 0 {
		return
	}
	in := pl.Inputs()
	pl.noteDeclared(pl.ns, in.Func, foreignNote{fixed: true}, func() { pl.shareNS(in.Func, pl.p.Name, nil) })
	for _, h := range in.Helpers {
		pl.declareFixed(pl.ns, h, pl.p.Name, nil)
		pl.shareAs(pl.e.Namespace, h, pl.p.Name, nil, meetsLocal)
	}
}

// InputSlot is field f of rec's slot in detail::<P>Inputs: the namespace named after rec's class, and in it f's member name f_; each is unique in its scope, so two valid fields never share a slot (log-2026-09-24 "gen/go runtime inputs landed").
func (pl *CppNamePlan) InputSlot(rec *Record, f *Field) (class, slot string) {
	return pl.TypeName(rec), pl.Member(f.Name)
}

// declareDependents declares each dependent type's class and branch enum in the namespace, the enumerators, then the class's members (CODEGEN.md §5.6).
func (pl *CppNamePlan) declareDependents() {
	for _, t := range pl.p.Types {
		d, ok := t.(*Dependent)
		if !ok {
			continue
		}
		n, origin := pl.Dependent(d), d.QName()
		pl.shareNS(n.Class, origin, d)
		pl.shareNSFrom(origin, d, derivation{d, func() string { return pl.Dependent(d).Branch }})
		enum, class := pl.scope(n.Branch), pl.classScope(n.Class, origin, d)
		pl.declareFixed(class, n.GetBranch, origin, d)
		for i, b := range n.Branches {
			at := origin + qnameSep + d.Branches[i].Name
			pl.declareOwn(enum, b.Enumerator, at, d)
			pl.declareOwn(class, b.As, at, d)
			if b.AsValue != "" {
				pl.declareOwn(class, b.AsValue, at, d)
			}
		}
		pl.declareFixed(class, n.Value, origin, d)
		if n.DefineValue != "" {
			pl.declareFixed(class, n.DefineValue, origin, d)
		}
	}
}

// declareInputSlots declares detail's input namespace and loaded flag, which other packages of the namespace share, a namespace per record with an input field, and in it each input's slot (CODEGEN.md §7.7).
func (pl *CppNamePlan) declareInputSlots(detail *nameScope) {
	recs := inputRecords(pl.p)
	if len(recs) == 0 {
		return
	}
	names := pl.Inputs()
	pl.innerFixed(detail, names.Namespace, pl.p.Name, meetsNever)
	pl.innerFixed(detail, names.Loaded, pl.p.Name, meetsNever)
	inputs := pl.scope(names.Namespace)
	for _, rec := range recs {
		class := pl.TypeName(rec)
		pl.declareFrom(inputs, rec.QName(), rec, derivation{rec, func() string { return pl.TypeName(rec) }})
		slots := pl.scope(names.Namespace + cppScope + class)
		for _, f := range inputFields(rec) {
			_, slot := pl.InputSlot(rec, f)
			pl.declareOwn(slots, slot, rec.QName()+qnameSep+f.Name, f)
		}
	}
}
