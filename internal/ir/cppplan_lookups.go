package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// CppFindBy are a @stable field's container lookup FindBy<F> and its two sorted indexes, by<F>Keys_ and by<F>_ (CODEGEN.md §5.9).
type CppFindBy struct {
	Func, Keys, Index string
}

// CppVector are a translated fn's conformance vector struct <Owner><Fn>Vector and its table k<Owner><Fn> (CONFORMANCE.md §7.2).
type CppVector struct {
	Struct, Table string
}

// CppEnumHelpers are an enum's k<E>Members, <E>FromWire and, with @codes, <E>FromCode (CODEGEN.md §5.2).
type CppEnumHelpers struct {
	Members, FromWire, FromCode string
}

// FindBy is the lookup and indexes of @stable field f (CODEGEN.md §5.9).
func (pl *CppNamePlan) FindBy(f *Field) CppFindBy {
	upper := cppUpperCamel(f.Name)
	by := cppByPrefix + upper
	return CppFindBy{Func: goFindByPrefix + upper, Keys: by + cppKeys + underscore, Index: by + underscore}
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
	return CppEnumHelpers{Members: cppConstPrefix + enum + goMembersSuffix, FromWire: enum + cppFromWire, FromCode: enum + goFromCodeSuffix}
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
			patterned = patterned || f.Pattern != nil
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
	pl.shareNS(in.Func, pl.p.Name, nil)
	for _, h := range in.Helpers {
		pl.declare(pl.ns, h, pl.p.Name, nil)
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
		pl.shareNSFrom(n.Class, origin, d, nil)
		pl.shareNSFrom(n.Branch, origin, d, d)
		enum, class := pl.scope(n.Branch), pl.classScope(n.Class, origin, d)
		pl.declare(class, n.GetBranch, origin, d)
		for i, b := range n.Branches {
			at := origin + qnameSep + d.Branches[i].Name
			pl.declare(enum, b.Enumerator, at, d)
			pl.declare(class, b.As, at, d)
			if b.AsValue != "" {
				pl.declare(class, b.AsValue, at, d)
			}
		}
		pl.declare(class, n.Value, origin, d)
	}
}

// declareInputSlots declares detail's input namespace and loaded flag, which other packages of the namespace share, a namespace per record with an input field, and in it each input's slot (CODEGEN.md §7.7).
func (pl *CppNamePlan) declareInputSlots(detail *nameScope) {
	recs := inputRecords(pl.p)
	if len(recs) == 0 {
		return
	}
	names := pl.Inputs()
	pl.shareInner(detail, names.Namespace, pl.p.Name, nil)
	pl.shareInner(detail, names.Loaded, pl.p.Name, nil)
	inputs := pl.scope(names.Namespace)
	for _, rec := range recs {
		class := pl.TypeName(rec)
		pl.declareFrom(inputs, class, rec.QName(), rec, rec)
		slots := pl.scope(names.Namespace + cppScope + class)
		for _, f := range inputFields(rec) {
			_, slot := pl.InputSlot(rec, f)
			pl.declare(slots, slot, rec.QName()+qnameSep+f.Name, f)
		}
	}
}
