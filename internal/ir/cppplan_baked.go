package ir

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// baked reports a baked emit, whose values gen/cpp builds into the program (CODEGEN.md §2.2, §5.9).
func (pl *CppNamePlan) baked() bool { return pl.e.Mode == ModeBaked }

// IDName is the id enum of a table of rec, <Rec>Id (CODEGEN.md §5.3, §7.3).
func (pl *CppNamePlan) IDName(rec *Record) string { return pl.TypeName(rec) + cppIDSuffix }

// IDMember is the enumerator of a table key, verbatim (CODEGEN.md §3.4, §5.3).
func (pl *CppNamePlan) IDMember(key string) string { return cppVerbatim(key) }

// Accessor is a value's accessor, Get + UpperCamel(v) or its @cpp(name:), then Key or Keys for a ref; the resolved accessor is the name without them (CODEGEN.md §3.3, §5.9).
func (pl *CppNamePlan) Accessor(v *Value) (getter, resolved string) {
	t, _ := goUnwrap(v.Type)
	resolved = cppOverride(v.Cpp, GoGet+cppUpperCamel(v.Name))
	return resolved + cppKeySuffix(t), resolved
}

// OwnerAccessor is the resolved accessor of the value r targets in its owner package's cpp emit, Get<V> or that value's @cpp(name:), named as the owner's own plan names it (Accessor; CODEGEN.md §3.3, §5.9).
func (pl *CppNamePlan) OwnerAccessor(r *RefTarget) string {
	_, resolved := pl.Accessor(&Value{Name: r.Value, Cpp: r.Cpp})
	return resolved
}

// DataMember is a value's or a package fn's member of detail::<P>Access::Data, verbatim (CODEGEN.md §7.3).
func (pl *CppNamePlan) DataMember(canon string) string { return cppVerbatim(canon) }

// RefData is the Data member holding a resolved ref value's entries beside its keys, <member>_ref (CODEGEN.md §5.8, §5.9).
func (pl *CppNamePlan) RefData(canon string) string { return cppVerbatim(canon) + cppRefData }

// CellsName is a constexpr package fn's cells, detail::k<Fn>Cells (CODEGEN.md §5.10, decision 293).
func (pl *CppNamePlan) CellsName(fn *ExportFn) string {
	return cppConstPrefix + pl.FnName(fn) + cppCellsSuffix
}

// IDTable is the table value whose id enum keys a ref of this emit, or nil when the key is not an enum: a ref into a public table value, of this package in a baked emit, or of another one whose cpp emit is baked or embedded in any mode (CODEGEN.md §5.3, §5.8; DECISIONS 312).
func (pl *CppNamePlan) IDTable(t TypeRef) *Value {
	r := t.Ref
	if t.Kind != types.Ref || r == nil || r.Coll != types.CollLet || r.Local || r.Keyed {
		return nil
	}
	if r.Pkg == pl.p.Name && !pl.baked() || r.Pkg != pl.p.Name && !enumIDs(pl.p, r.Pkg) {
		return nil
	}
	rec, ok := r.Elem.(*Record)
	if !ok {
		return nil
	}
	return &Value{Name: r.Value, Type: TypeRef{Kind: types.Table, Elem: &TypeRef{Kind: types.Record, Named: rec}}}
}

// enumIDs reports an imported package whose cpp emit keys its tables by an id enum: baked or embedded (CODEGEN.md §2.2).
func enumIDs(p *Package, pkg string) bool {
	i := slices.IndexFunc(p.Imports, func(r *PackageRef) bool { return r.Name == pkg })
	if i < 0 {
		return false
	}
	j := slices.IndexFunc(p.Imports[i].Emits, func(e *Emit) bool { return e.Target == TargetCpp })
	return j >= 0 && (p.Imports[i].Emits[j].Mode == ModeBaked || p.Imports[i].Emits[j].Mode == ModeEmbedded)
}

// Constexpr reports a package-level precomputed or lookup fn gen/cpp defines in the header as an `inline constexpr` function: its result is a constexpr scalar, an integer, Float, Bool, String or a string literal union, Duration, an enum or a table id with an id enum here (CODEGEN.md §5.10, decisions 293, 296).
func (pl *CppNamePlan) Constexpr(fn *ExportFn) bool {
	if !pl.baked() || fn.Kind == FnTranslated {
		return false
	}
	switch fn.Result.Kind {
	case types.Bool, types.Int, types.Float, types.String, types.Duration, types.Enum, types.LitUnion:
		return true
	case types.Ref:
		return pl.IDTable(fn.Result) != nil
	default:
		return false
	}
}

// declareBaked declares what a baked emit adds to the namespace (CODEGEN.md §5.3, §5.9, §5.10, §7.3): the id enum of every public table value with its members and <Rec>IdFromWire, each value's accessors, then each stored package fn.
func (pl *CppNamePlan) declareBaked() {
	for _, v := range pl.p.Values {
		rec := tableRecord(v)
		if rec == nil {
			continue
		}
		origin := pl.valueOrigin(v)
		pl.shareNSFrom(origin, v, derivation{rec, func() string { return pl.IDName(rec) }})
		pl.shareNSFrom(origin, v, derivation{rec, func() string { return pl.IDName(rec) + cppFromWire }})
		sc := pl.scope(pl.IDName(rec))
		for _, id := range v.IDs {
			pl.declare(sc, pl.IDMember(id), origin+qnameSep+id, v)
		}
	}
	for _, v := range pl.values {
		getter, resolved := pl.Accessor(v)
		t, _ := goUnwrap(v.Type)
		if pl.Resolves(t, nil) {
			pl.shareNS(resolved, pl.valueOrigin(v), v)
		}
		if getter != resolved || !pl.Resolves(t, nil) {
			pl.shareNS(getter, pl.valueOrigin(v), v)
		}
	}
	for _, fn := range pl.p.Fns {
		if fn.Kind != FnTranslated {
			pl.shareNS(pl.FnName(fn), pl.fnOrigin(fn), fn)
		}
	}
}

// declareBakedDetail declares detail's constexpr cells, and the access struct's Data, Build and Get with Data's members: each value, a ref value's resolved entries, each package fn stored there (CODEGEN.md §5.10, §7.3).
func (pl *CppNamePlan) declareBakedDetail(detail, access *nameScope) {
	for _, fn := range pl.p.Fns {
		if pl.Constexpr(fn) {
			pl.declareInner(detail, pl.CellsName(fn), pl.fnOrigin(fn), fn)
		}
	}
	for _, n := range cppAccessMembers {
		pl.declare(access, n, pl.p.Name, nil)
	}
	data := pl.scope(cppDataStruct)
	for _, v := range pl.values {
		pl.declare(data, pl.DataMember(v.Name), pl.valueOrigin(v), v)
		if t, _ := goUnwrap(v.Type); pl.Resolves(t, nil) {
			pl.declare(data, pl.RefData(v.Name), pl.valueOrigin(v), v)
		}
	}
	for _, fn := range pl.p.Fns {
		if fn.Kind == FnTranslated || pl.Constexpr(fn) {
			continue
		}
		pl.declare(data, pl.DataMember(fn.Name), pl.fnOrigin(fn), fn)
	}
}

// resolvesBaked reports a ref of a baked emit that gets a resolved getter: one into a table or keyed list value the emit selects, of this package (CODEGEN.md §5.8).
func (pl *CppNamePlan) resolvesBaked(r *RefTarget) bool {
	i := slices.IndexFunc(pl.values, func(v *Value) bool { return v.Name == r.Value })
	return i >= 0 && IsContainer(pl.values[i])
}

// declareJSONHooks declares the deleted nlohmann hooks to_json and from_json that gen/cpp writes for the id enums, shared as overloads by the packages emitted into one namespace (CODEGEN.md §3.5, §5.3; DECISIONS 379).
func (pl *CppNamePlan) declareJSONHooks() {
	if !pl.tablesOwnToWire() {
		return
	}
	for _, n := range cppJSONHooks {
		pl.declare(pl.ns, n, pl.p.Name, nil)
		pl.shareAs(pl.e.Namespace, n, pl.p.Name, nil, meetsOverload)
	}
}

// tablesOwnToWire reports a baked package with a public table and no enum or variant: its id enums still overload ToWire (CODEGEN.md §5.3).
func (pl *CppNamePlan) tablesOwnToWire() bool {
	return pl.baked() && slices.ContainsFunc(pl.p.Values, func(v *Value) bool { return tableRecord(v) != nil })
}

// CppBulkArray is a baked Build's n-th array, kBulk<N> in detail's unnamed namespace (CODEGEN.md §3.5, §7.3).
func CppBulkArray(n int) string { return cppBulkPrefix + strconv.Itoa(n) }

// CppBuildPart is <P>Access's n-th part of Build, Build<N> (CODEGEN.md §3.5, §7.3).
func CppBuildPart(n int) string { return cppBuild + strconv.Itoa(n) }

// ElemsName is a long list or map constant's n-th array in detail: k<NAME>Elems, then k<NAME>Elems<N> for a nested one's further arrays (CODEGEN.md §3.5, §5.1).
func (pl *CppNamePlan) ElemsName(c *Const, n int) string {
	name := cppConstPrefix + pl.ConstName(c) + cppElemsSuffix
	if n == 0 {
		return name
	}
	return name + strconv.Itoa(n)
}

// numbered reports name as prefix and a number strconv writes so: a name of CppBulkArray's or CppBuildPart's family.
func numbered(name, prefix string) bool {
	rest, ok := strings.CutPrefix(name, prefix)
	n, err := strconv.Atoi(rest)
	return ok && err == nil && n >= 0 && strconv.Itoa(n) == rest
}

// reserveBulk is E8005 for a namespace name a kBulk<N> array of detail or a Build<N> part of the access struct hides where Build names it, whatever the data's size (CODEGEN.md §3.5, DECISIONS 392).
func (pl *CppNamePlan) reserveBulk() {
	for _, name := range slices.Sorted(maps.Keys(pl.ns.names)) {
		if numbered(name, cppBulkPrefix) || numbered(name, cppBuild) {
			pl.collide(pl.ns, name, pl.ns.names[name], pl.p.Name, pl.ns.items[name]) // at the user's declaration
		}
	}
}

// declareElems declares each long list or map constant's detail::k<NAME>Elems, shared by the namespace's headers, and is E8005 for a namespace name of its numbered family (CODEGEN.md §5.1).
func (pl *CppNamePlan) declareElems(detail *nameScope) {
	for _, c := range pl.p.Consts {
		if !bulkConst(c) {
			continue
		}
		first := pl.ElemsName(c, 0)
		pl.shareInner(detail, first, c.Name, c)
		for _, name := range slices.Sorted(maps.Keys(pl.ns.names)) {
			if numbered(name, first) {
				pl.collide(pl.ns, name, pl.ns.names[name], c.Name, pl.ns.items[name])
			}
		}
	}
}

// bulkConst reports a list or map constant of at least CppBulkMinElems elements across its levels.
func bulkConst(c *Const) bool { return BulkCount(c.Type, c.V) >= CppBulkMinElems }

// BulkCount is the elements of v of type t across every level of nested lists and maps: what
// gen/cpp weighs against CppBulkMinElems before reading a literal from arrays (DECISIONS 392).
func BulkCount(t TypeRef, v value.Value) int {
	if t.Elem == nil || t.Kind == types.List && t.KeyedBy != nil || t.Kind != types.List && t.Kind != types.Map {
		return 0
	}
	var items []value.Value
	switch x := v.(type) {
	case *value.List:
		items = x.Elems
	case *value.Map:
		items = x.Vals
	default:
		return 0
	}
	n := len(items)
	for _, it := range items {
		n += BulkCount(*t.Elem, it)
	}
	return n
}
