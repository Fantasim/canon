package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
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

// DataMember is a value's or a package fn's member of detail::<P>Access::Data, verbatim (CODEGEN.md §7.3).
func (pl *CppNamePlan) DataMember(canon string) string { return cppVerbatim(canon) }

// RefData is the Data member holding a resolved ref value's entries beside its keys, <member>_ref (CODEGEN.md §5.8, §5.9).
func (pl *CppNamePlan) RefData(canon string) string { return cppVerbatim(canon) + cppRefData }

// CellsName is a constexpr package fn's cells, detail::k<Fn>Cells (CODEGEN.md §5.10, decision 293).
func (pl *CppNamePlan) CellsName(fn *ExportFn) string {
	return cppConstPrefix + pl.FnName(fn) + cppCellsSuffix
}

// IDTable is the table value whose id enum keys a ref of this emit, or nil when the key is not an enum: a ref into a public table value, of this package or of one whose cpp emit is baked or embedded (CODEGEN.md §5.3, §5.8).
func (pl *CppNamePlan) IDTable(t TypeRef) *Value {
	r := t.Ref
	if !pl.baked() || t.Kind != types.Ref || r == nil || r.Coll != types.CollLet || r.Local || r.Keyed {
		return nil
	}
	if r.Pkg != pl.p.Name && !enumIDs(pl.p, r.Pkg) {
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

// tablesOwnToWire reports a baked package with a public table and no enum or variant: its id enums still overload ToWire (CODEGEN.md §5.3).
func (pl *CppNamePlan) tablesOwnToWire() bool {
	return pl.baked() && slices.ContainsFunc(pl.p.Values, func(v *Value) bool { return tableRecord(v) != nil })
}
