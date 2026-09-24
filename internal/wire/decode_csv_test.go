package wire_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// cells splits a CSV text without quotes into cells located in file f, as load's reader does.
func cells(f *source.File) [][]wire.Cell {
	var rows [][]wire.Cell
	pos := 0
	for _, line := range strings.SplitAfter(string(f.Content), "\n") {
		text := strings.TrimSuffix(line, "\n")
		if text == "" {
			pos += len(line)
			continue
		}
		var row []wire.Cell
		start := pos
		for _, c := range strings.Split(text, ",") {
			span := source.Span{File: f.ID, Start: source.Pos(start), End: source.Pos(start + len(c))}
			row = append(row, wire.Cell{Text: c, Span: span})
			start += len(c) + 1
		}
		rows = append(rows, row)
		pos += len(line)
	}
	return rows
}

// decodeCSV decodes text as c.csv with a header against typ.
func decodeCSV(t *testing.T, dec wire.Decoder, text string, typ types.Type) decoded {
	t.Helper()
	fs := &source.FileSet{}
	f, err := fs.Add("c.csv", "/p/c.csv", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	dec.Bag, dec.Pkg = diag.NewBag(fs, "p"), "p"
	rows := cells(f)
	v, ok, err := dec.CSV(context.Background(), rows[0], rows[1:], typ)
	return decoded{v: v, ok: ok, err: err, findings: short(fs, dec.Bag.Findings())}
}

// cellRow is a record of every kind of cell WIRE.md §6.6 lists.
func cellRow(h *host) *types.RecordType {
	dur := field("delay", types.DurationType)
	dur.Unit = types.UnitS
	b := field("on", types.BoolType)
	b.Enc = types.EncInt
	marker := field("m", opt(types.IntType))
	marker.NoneWire = []byte("-1")
	return record("p", "Row", field("i", types.Int16Type), field("f", types.Float32Type), dur, b,
		field("s", types.StringType), field("e", element), field("r", &types.RefType{Target: intKeyed}),
		field("u", &types.LitUnionType{Of: tone, Literals: []string{"any"}}), marker,
		h.withDefault(field("d", types.IntType), num(9)))
}

var csvCases = []struct {
	rule, text, want string
}{
	{"§6.6 cells", "i,f,delay,on,s,e,r,u,m\n-0x1F,1.5e3,0.5,1,a b,2,7,series-1,-1\n",
		`[Row{i: -31, f: 1500, delay: 500ms, on: true, s: "a b", e: WATER, r: 7, u: series_1, m: none, d: 9}]`},
	{"§6.6 empty cells", "i,f,delay,on,s,e,r,u,m,d\n1_000,0x10,2,0,x,1,0,any,,\n",
		`[Row{i: 1000, f: 16, delay: 2s, on: false, s: "x", e: FIRE, r: 0, u: "any", m: none, d: 9}]`},
	{"§6.6 no parse", "i,f,delay,on,s,e,r,u\n1__0,1.,x,true,s,1.0,a,any\n",
		findings(at(diag.E7108, "2:1", ""), at(diag.E7108, "2:6", ""), at(diag.E7108, "2:9", ""), at(diag.E7108, "2:11", ""), at(diag.E7108, "2:18", ""), at(diag.E7108, "2:22", ""))},
	{"§6.6 ranges", "i,f,delay,on,s,e,r,u\n40000,1e39,0.0001,1,s,9,1,x\n", findings(at(diag.E3201, "2:1", ""), at(diag.E3202, "2:7", ""), at(diag.E3203, "2:12", ""), at(diag.E7111, "2:23", ""), at(diag.E7111, "2:27", ""))},
	{"§6.6 optional cell", "i,f,delay,on,s,e,r,u,m\n1,1,1,1,s,1,1,any,3\n",
		`[Row{i: 1, f: 1, delay: 1s, on: true, s: "s", e: FIRE, r: 1, u: "any", m: 3, d: 9}]`},
	{"§6.6 integer ref keys", "i,f,delay,on,s,e,r,u\n1,1,1,1,s,1,1_000,any\n2,1,1,1,s,1,0x1F,any\n3,1,1,1,s,1,-0,any\n",
		findings(at(diag.E7108, "2:13", ""), at(diag.E7108, "3:13", ""))},
	{"§6.6 leading zero", "i,f,delay,on,s,e,r,u\n007,0.5,1,1,s,1,1,any\n", at(diag.E7108, "2:1", "")},
	{"§6.6 header", "i,i,zz,f,delay,on,s,e,r\n", findings(at(diag.E3302, "1:1", ""), at(diag.E7104, "1:3", ""), at(diag.E3301, "1:5", ""))},
	{"§6.6 empty required cell", "i,f,delay,on,s,e,r,u\n,1,1,1,,1,1,any\n", findings(at(diag.E3302, "2:1", ""), at(diag.E3302, "2:8", ""))},
}

// WIRE.md §6.6: cells are Canon literals of their field's type; an empty cell is absent.
func TestDecodeCSV(t *testing.T) {
	h := newHost()
	rows := listOf(cellRow(h))
	for _, c := range csvCases {
		if got := decodeCSV(t, wire.Decoder{Host: h}, c.text, rows).text(); got != c.want {
			t.Errorf("%s: %q, want %q", c.rule, got, c.want)
		}
	}
}

// WIRE.md §6.6: `$id` keys a table, a list column is E7116, a headerless file [[String]].
func TestDecodeCSVShapes(t *testing.T) {
	table := &types.TableType{Elem: status}
	if got := decodeCSV(t, wire.Decoder{}, "$id,label\nopen,O\nbad-key,B\n", table).text(); got != at(diag.E7114, "3:1", "") {
		t.Errorf("table keys: %q", got)
	}
	if got := decodeCSV(t, wire.Decoder{}, "label\nO\n", table).text(); got != at(diag.E3302, "1:1", "") {
		t.Errorf("table without $id: %q", got)
	}
	if got := decodeCSV(t, wire.Decoder{Partial: true}, "$id,label,extra\nopen,O,x\n", table).text(); got != `{open: Status{label: "O"}}` {
		t.Errorf("partial table: %q", got)
	}
	nested := record("p", "L", field("xs", listOf(types.IntType)))
	if got := decodeCSV(t, wire.Decoder{}, "xs\n1\n", listOf(nested)).text(); got != findings(at(diag.E3302, "1:1", ""), at(diag.E7116, "1:1", "")) {
		t.Errorf("list column: %q", got)
	}
	if got := decodeCSV(t, wire.Decoder{}, "$id,label\nopen,A\nopen,B\n", table).text(); got != at(diag.E3102, "3:1", "") {
		t.Errorf("repeated $id: %q", got)
	}
	variant := variantOf("p", "V", "kind", &types.CaseType{Name: "a"})
	inline := field("v", variant)
	inline.Inline = true
	if got := decodeCSV(t, wire.Decoder{}, "v\na\n", listOf(record("p", "I", inline))).text(); got != findings(at(diag.E3301, "1:1", ""), at(diag.E3302, "1:1", "")) {
		t.Errorf("a column naming an inline field: %q", got)
	}
	fs := &source.FileSet{}
	f, _ := fs.Add("s.csv", "/p/s.csv", []byte("a,b\nc,d\n"))
	dec := wire.Decoder{Bag: diag.NewBag(fs, "p")}
	v, ok, err := dec.CSV(context.Background(), nil, cells(f), listOf(listOf(types.StringType)))
	if !ok || err != nil || v.CanonText() != `[["a", "b"], ["c", "d"]]` {
		t.Errorf("[[String]]: %v %v %v", v, ok, err)
	}
}

// examples/features: the CSV and JSON data files decode against their declarations.
func TestDecodeFeatureData(t *testing.T) {
	h := newHost()
	level := record("features.csv", "LevelRow", field("level", types.IntType), field("exp", types.IntType, "exp_to_next"),
		h.withDefault(field("deathPenalty", types.IntType, "death_penalty"), num(0)))
	text, err := os.ReadFile("../../examples/features/csv/data/exp_curve.csv")
	if err != nil {
		t.Fatal(err)
	}
	got := decodeCSV(t, wire.Decoder{Host: h}, string(text), keyed(level, 0))
	if !got.ok || !strings.HasPrefix(got.v.CanonText(), "[LevelRow{level: 1, exp: 14, deathPenalty: 0}") {
		t.Errorf("exp_curve.csv: %s", got.text())
	}
	country := record("features.embedded", "Country", field("code", types.StringType), field("name", types.StringType),
		field("vat", types.FloatType), h.withDefault(field("cards", types.BoolType), boolean(true)))
	v := decodeFile(t, wire.Decoder{Host: h}, "../../examples/features/embedded/data/countries.json", "", keyed(country, 0))
	if !strings.Contains(v.CanonText(), `Country{code: "CH", name: "Switzerland", vat: 8.1, cards: false}`) {
		t.Errorf("countries.json: %s", v.CanonText())
	}
	rank := codes(enum("features.codes", "GuildRank", "master", "kingpin", "captain", "supporter", "rookie"), 1, 2, 3, 4, 5)
	rank.WireCodes = true
	rights := record("features.codes", "RankRights", field("rank", rank), field("invitesPerDay", types.IntType),
		h.withDefault(field("bank", types.BoolType), boolean(false)))
	v = decodeFile(t, wire.Decoder{Host: h}, "../../examples/features/codes/data/guild_rights.json", "", keyed(rights, 0))
	if id := v.(*value.List).Elems[4].(*value.Record).Ident; id == nil || id.Key.S != "rookie" {
		t.Errorf("guild_rights.json identity: %+v", id)
	}
	items := &types.Collection{Kind: types.CollDefines, Pkg: "features.legacycpp", Name: "items"}
	kind1 := codes(enum("features.legacycpp", "Kind1", "IK1_GOLD", "IK1_WEAPON", "IK1_ARMOR", "IK1_GENERAL"), 0, 1, 2, 3)
	permanent := h.withDefault(field("permanent", types.BoolType, "bPermanence"), boolean(true))
	permanent.Enc = types.EncInt
	prop := record("features.legacycpp", "Prop", field("id", &types.RefType{Target: items}, "dwID"), field("kind1", kind1, "dwItemKind1"),
		h.withDefault(field("level", types.IntType, "dwItemLV"), num(1)), h.withDefault(field("stackMax", types.IntType, "dwPackMax"), num(1)), permanent)
	v = decodeFile(t, wire.Decoder{Host: h}, "../../examples/features/legacycpp/data/props.json", "items", keyed(prop, 0))
	if want := "Prop{id: II_GEN_FOO_INS_HOTDOG, kind1: IK1_GENERAL, level: 52, stackMax: 9999, permanent: false}"; !strings.Contains(v.CanonText(), want) {
		t.Errorf("props.json: %s", v.CanonText())
	}
}
