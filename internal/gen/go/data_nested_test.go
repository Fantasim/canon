package gogen_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// boxes builds boxPackage: a nested keyed list (CODEGEN.md §4.2) per key type TYPES.md §9.1 allows.
type boxes struct {
	item, box                *ir.Record
	size                     *ir.Enum
	slot, crate              *ir.Record
	grade                    *ir.Enum
	bin, vault               *ir.Record
	label, ticket, ticketBox *ir.Record
	labels                   *ir.Value
	award, trophy            *ir.Record
}

// boxPackage is a data-mode package exercising every keyed-by-field key type (see boxes' doc).
func boxPackage() *ir.Package {
	b := &boxes{}
	basep := base()
	b.items()
	b.crates()
	b.vaults()
	b.tickets()
	b.trophies(basep)
	bv := &ir.Value{Name: "box", Schema: "boxes.Box@00000001", Type: typed(b.box, types.Record)}
	cv := &ir.Value{Name: "crate", Schema: "boxes.Crate@00000002", Type: typed(b.crate, types.Record)}
	vv := &ir.Value{Name: "vault", Schema: "boxes.Vault@00000003", Type: typed(b.vault, types.Record)}
	tv := &ir.Value{Name: "ticketBox", Schema: "boxes.TicketBox@00000004", Type: typed(b.ticketBox, types.Record)}
	yv := &ir.Value{Name: "trophy", Schema: "boxes.Trophy@00000006", Type: typed(b.trophy, types.Record)}
	return &ir.Package{
		Name: "boxes", Dir: "boxes",
		Imports: []*ir.PackageRef{{Name: basePkg, Dir: "demo/base", Emits: basep.Emits}},
		Types: []ir.Type{
			b.item, b.box, b.size, b.slot, b.crate, b.grade, b.bin, b.vault, b.label, b.ticket, b.ticketBox, b.award, b.trophy,
		},
		Values: []*ir.Value{bv, cv, vv, b.labels, tv, yv},
		Emits:  []*ir.Emit{goData("boxes", "boxes")},
	}
}

// items: box.items is keyed by an Int field written at a nested path, `@json(path: "legacy.a")`.
func (b *boxes) items() {
	b.item = &ir.Record{Pkg: "boxes", Name: "Item", Fields: []*ir.Field{{Name: "a", WirePath: []string{"legacy", "a"}, Type: intT}}}
	elem := typed(b.item, types.Record)
	b.box = &ir.Record{Pkg: "boxes", Name: "Box", Fields: []*ir.Field{{
		Name: "items", WirePath: []string{"items"},
		Type: ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "a", WirePath: []string{"legacy", "a"}}},
	}}}
}

// crates: crate.slots is keyed by a plain enum field.
func (b *boxes) crates() {
	b.size = &ir.Enum{Pkg: "boxes", Name: "Size", Members: []*ir.EnumMember{
		{Name: "small", Wire: "small"}, {Name: "large", Wire: "large", Index: 1},
	}}
	b.slot = &ir.Record{Pkg: "boxes", Name: "Slot", Fields: []*ir.Field{wired("size", "size", "", typed(b.size, types.Enum))}}
	selem := typed(b.slot, types.Record)
	b.crate = &ir.Record{Pkg: "boxes", Name: "Crate", Fields: []*ir.Field{{
		Name: "slots", WirePath: []string{"slots"},
		Type: ir.TypeRef{Kind: types.List, Elem: &selem, KeyedBy: &ir.KeyField{Name: "size", WirePath: []string{"size"}}},
	}}}
}

// vaults: vault.bins is keyed by a @json(codes) enum field.
func (b *boxes) vaults() {
	b.grade = &ir.Enum{Pkg: "boxes", Name: "Grade", Codes: &u8T, JSONCodes: true, Members: []*ir.EnumMember{
		{Name: "bronze", Wire: "bronze", Code: 1}, {Name: "gold", Wire: "gold", Index: 1, Code: 3},
	}}
	b.bin = &ir.Record{Pkg: "boxes", Name: "Bin", Fields: []*ir.Field{wired("grade", "grade", "", typed(b.grade, types.Enum))}}
	belem := typed(b.bin, types.Record)
	b.vault = &ir.Record{Pkg: "boxes", Name: "Vault", Fields: []*ir.Field{{
		Name: "bins", WirePath: []string{"bins"},
		Type: ir.TypeRef{Kind: types.List, Elem: &belem, KeyedBy: &ir.KeyField{Name: "grade", WirePath: []string{"grade"}}},
	}}}
}

// tickets: ticketBox.tickets is keyed by a ref field, into the top-level keyed list labels.
func (b *boxes) tickets() {
	b.label = &ir.Record{Pkg: "boxes", Name: "Label", Fields: []*ir.Field{wired("name", "name", "", strT)}}
	lelem := typed(b.label, types.Record)
	b.labels = &ir.Value{
		Name: "labels", Schema: "boxes.Label@00000005",
		Type: ir.TypeRef{Kind: types.List, Elem: &lelem, KeyedBy: &ir.KeyField{Name: "name", WirePath: []string{"name"}}},
	}
	b.ticket = &ir.Record{Pkg: "boxes", Name: "Ticket", Fields: []*ir.Field{wired("label", "label", "", refT("boxes", "labels", b.label, true))}}
	telem := typed(b.ticket, types.Record)
	b.ticketBox = &ir.Record{Pkg: "boxes", Name: "TicketBox", Fields: []*ir.Field{{
		Name: "tickets", WirePath: []string{"tickets"},
		Type: ir.TypeRef{Kind: types.List, Elem: &telem, KeyedBy: &ir.KeyField{Name: "label", WirePath: []string{"label"}}},
	}}}
}

// trophies: trophy.awards is keyed by a ref into an imported baked table whose ids are an enum
// (refKeyToken's isTableRef+enumIDs branch: the id enum has String(), never Wire()).
func (b *boxes) trophies(basep *ir.Package) {
	rank := basep.Types[1].(*ir.Record)
	b.award = &ir.Record{Pkg: "boxes", Name: "Award", Fields: []*ir.Field{wired("rank", "rank", "", refT(basep.Name, "ranks", rank, false))}}
	aelem := typed(b.award, types.Record)
	b.trophy = &ir.Record{Pkg: "boxes", Name: "Trophy", Fields: []*ir.Field{{
		Name: "awards", WirePath: []string{"awards"},
		Type: ir.TypeRef{Kind: types.List, Elem: &aelem, KeyedBy: &ir.KeyField{Name: "rank", WirePath: []string{"rank"}}},
	}}}
}

// A nested keyed list also refuses a duplicate key: an Int, a plain enum, a @codes enum and a ref.
func TestDataBoxesRuns(t *testing.T) {
	runData(t, generateData(t, base(), boxPackage()), "boxes/out/go", "testdata/smoke/data_boxes_test.go", readData(t, "testdata/datafiles/boxes"))
}
