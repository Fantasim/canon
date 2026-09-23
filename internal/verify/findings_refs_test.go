package verify_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// statuses is taxonomy.canon's Status, trimmed: a label and the statuses it may move to.
type statuses struct {
	fx     *fixture
	status *types.RecordType
	next   *types.ListType
	table  *types.TableType
	coll   *types.Collection
}

func newStatuses(fx *fixture) *statuses {
	s := &statuses{fx: fx}
	s.status = &types.RecordType{Pkg: pkg, Name: "Status"}
	s.coll = collection("statuses", s.status)
	ref := &types.RefType{Target: s.coll}
	if strings.Contains(fx.src, "ref Status") {
		fx.written("ref Status", ref)
	}
	s.next = &types.ListType{Elem: ref}
	s.status.Fields = []*types.Field{{Name: "label", Type: types.StringType}, {Name: "next", Index: 1, Type: s.next}}
	s.table = &types.TableType{Elem: s.status, Stable: true}
	return s
}

// entry is `key { label: "label", next: [refs…] }`.
func (s *statuses) entry(key, label string, next ...string) *value.Record {
	refs := &value.List{T: s.next, P: s.fx.lit("[", key+" {", "next: ")}
	for _, n := range next {
		refs.Elems = append(refs.Elems, &value.Ref{T: s.next.Elem, Key: value.Key{S: n}, P: s.fx.lit(n, key+" {", "next: ")})
	}
	return s.fx.entry(s.coll, key, s.status, str(label, types.StringType, s.fx.lit(`"`+label+`"`)), refs)
}

func (s *statuses) let(entries ...*value.Record) {
	s.fx.let("statuses", s.table, &value.Table{T: s.table, Entries: entries, P: s.fx.lit("{", "let statuses")})
}

// TYPES.md §10.3: a ref names an entry of its target.
func danglingCase(fx *fixture) {
	s := newStatuses(fx)
	s.let(s.entry("open", "Open", "taken", "closed"), s.entry("taken", "Taken", "open"))
	initial := &value.Ref{T: s.next.Elem, Key: value.Key{S: "draft"}, P: fx.lit("draft")}
	initial.T = fx.written("ref Status", &types.RefType{Target: s.coll}, "let initialStatus")
	fx.let("initialStatus", initial.T, initial)
	fx.verify("statuses", "initialStatus")
}

// TYPES.md §10.3, LOCK.md §4.3: only a retired entry may name a retired one.
func retiredRefCase(fx *fixture) {
	s := newStatuses(fx)
	duplicate := s.entry("duplicate", "Duplicate", "open", "gone")
	duplicate.Ident.Retired = true
	s.let(s.entry("open", "Open", "taken", "duplicate"), s.entry("taken", "Taken", "open"), duplicate)
	last := fx.written("ref Status", &types.RefType{Target: s.coll}, "let lastStatus")
	fx.let("lastStatus", last, &value.Ref{T: last, Key: value.Key{S: "duplicate"}, P: fx.lit("duplicate", "let lastStatus")})
	fx.verify("statuses", "lastStatus")
}

// TYPES.md §8.1: a retired member or case is not used in a value, but in a retired entry.
func retiredUseCase(fx *fixture) {
	element := &types.EnumType{Pkg: pkg, Name: "Element", Decl: fx.decl("Element").(*syntax.EnumDecl), Members: []*types.Member{
		{Name: "FIRE", Index: 0}, {Name: "WIND", Index: 1, Retired: true},
	}}
	reward := &types.VariantType{Pkg: pkg, Name: "Reward", Decl: fx.decl("Reward").(*syntax.VariantDecl)}
	gold := &types.CaseType{Variant: reward, Name: "gold", Index: 1, Retired: true, Fields: []*types.Field{field("amount", types.IntType)}}
	reward.Cases = []*types.CaseType{{Variant: reward, Name: "item"}, gold}
	spell := record("Spell", field("element", element), field("reward", reward))
	spells := &types.TableType{Elem: spell, Stable: true}
	coll := collection("spells", spell)
	cast := func(key string, retired bool) *value.Record {
		e := fx.entry(coll, key, spell,
			&value.Member{Enum: element, Index: 1, P: fx.lit("WIND", key+" {")},
			&value.Record{T: gold, Fields: []value.Value{integer(3, types.IntType, fx.lit("3", key+" {"))}, P: fx.lit("gold", key+" {")})
		e.Ident.Retired = retired
		return e
	}
	fx.let("spells", spells, &value.Table{T: spells, Entries: []*value.Record{cast("gust", false), cast("storm", true)}, P: fx.lit("{", "let spells")})
	fx.verify("spells")
}

// TYPES.md §13.4: a clean path, an allowed extension, an existing file, case included.
func assetCase(sword, shield string) func(fx *fixture) {
	return func(fx *fixture) {
		fx.assets = assets{"@resource/Icon/Item": {"sword.png", "shield.png"}}
		icon := fx.written(`asset("@resource/Icon/Item", ext: [png, dds])`, &types.Refined{
			Of: types.StringType, Asset: &types.AssetSpec{Root: "@resource/Icon/Item", Exts: []string{"png", "dds"}},
		})
		item := record("Item", field("icon", icon))
		items := &types.TableType{Elem: item, Stable: true}
		coll := collection("items", item)
		entry := func(key, path string) *value.Record {
			return fx.entry(coll, key, item, str(path, icon, fx.lit(`"`+path+`"`)))
		}
		fx.let("items", items, &value.Table{T: items, P: fx.lit("{", "let items"), Entries: []*value.Record{
			entry("sword", sword), entry("shield", shield),
		}})
		fx.verify("items")
	}
}
