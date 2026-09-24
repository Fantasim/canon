package wire_test

import (
	"context"
	"math"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// roundTrip encodes v as a record's data file, reads its "value" back as t and compares.
func roundTrip(t *testing.T, v value.Value, typ types.Type) {
	t.Helper()
	b, err := (&wire.Document{Schema: "p.R@00000000", Kind: types.Record, V: v}).Encode()
	if err != nil {
		t.Fatalf("encode %s: %v", v.CanonText(), err)
	}
	fs := &source.FileSet{}
	f, _ := fs.Add("r.json", "/p/r.json", b)
	bag := diag.NewBag(fs, "p")
	root, err := jsonsrc.Parse(f, bag)
	if err != nil {
		t.Fatalf("parse %s: %v", b, err)
	}
	dec := wire.Decoder{Bag: bag, Pkg: "p"}
	got, ok, err := dec.Decode(context.Background(), wire.Selection{Node: root.Members[1].Value}, typ)
	if !ok || err != nil {
		t.Fatalf("decode %s: %v %v", b, err, short(fs, bag.Findings()))
	}
	if !sameValue(got, v) {
		t.Fatalf("decode(encode(v)) differs:\n%s\n%s\nfrom %s", got.CanonText(), v.CanonText(), b)
	}
}

// sameValue is structural equality of decoded values: kinds, fields, elements in order, entry keys
// and retirement; identities compare by key, since a decoder makes its own collections.
func sameValue(a, b value.Value) bool {
	switch x := a.(type) {
	case *value.Record:
		y, ok := b.(*value.Record)
		return ok && sameRecord(x, y)
	case *value.List:
		y, ok := b.(*value.List)
		return ok && sameValues(x.Elems, y.Elems)
	case *value.Map:
		y, ok := b.(*value.Map)
		return ok && sameValues(x.Keys, y.Keys) && sameValues(x.Vals, y.Vals)
	case *value.Table:
		y, ok := b.(*value.Table)
		rx, ry := make([]value.Value, 0), make([]value.Value, 0)
		for i := range x.Entries {
			rx = append(rx, x.Entries[i])
		}
		for i := range y.Entries {
			ry = append(ry, y.Entries[i])
		}
		return ok && sameValues(rx, ry)
	case *value.Ref:
		y, ok := b.(*value.Ref)
		return ok && x.Key == y.Key
	case *value.Float:
		y, ok := b.(*value.Float)
		return ok && x.V == y.V
	}
	return value.Equal(a, b)
}

func sameRecord(x, y *value.Record) bool {
	if declOf(x.T) != declOf(y.T) || len(x.Fields) != len(y.Fields) {
		return false
	}
	if (x.Ident == nil) != (y.Ident == nil) || x.Ident != nil && (x.Ident.Key != y.Ident.Key || x.Ident.Retired != y.Ident.Retired) {
		return false
	}
	for i := range x.Fields {
		if (x.Fields[i] == nil) != (y.Fields[i] == nil) || x.Fields[i] != nil && !sameValue(x.Fields[i], y.Fields[i]) {
			return false
		}
	}
	return true
}

func declOf(t types.Type) types.Type {
	if a, ok := t.Base().(*types.AppliedRecord); ok {
		return a.Rec
	}
	return t.Base()
}

func sameValues(a, b []value.Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameValue(a[i], b[i]) {
			return false
		}
	}
	return true
}

// WIRE.md §5, IMPLEMENTATION-PLAN.md §7.7: the samples the encoder writes decode back.
func TestDecodeEncoderSamples(t *testing.T) {
	skill, element, flag := fpdemoSkill()
	weights := &value.Map{T: skill.Fields[8].Type, Keys: []value.Value{member(element, 0), member(element, 1)}, Vals: []value.Value{flt(0.5), flt(1.25)}}
	row := rec(skill, str("SI_FIREBALL"), num(12), num(0), member(element, 0), list(flag, member(flag, 0), member(flag, 2)),
		boolean(false), &value.Str{V: "both", T: skill.Fields[6].Type}, dur(2000), weights)
	roundTrip(t, row, skill)
	row.Fields[3], row.Fields[4], row.Fields[7] = none(skill.Fields[3].Type), list(flag), none(skill.Fields[7].Type)
	roundTrip(t, row, skill)
	table, _, _ := flowStatuses()
	holder := record("flow", "Holder", field("statuses", table.T))
	roundTrip(t, rec(holder, table), holder)
	tree, coll := talentFixture()
	nodes := list(tree.Fields[0].Type.(*types.ListType).Elem)
	nodes.T = tree.Fields[0].Type
	for i, parent := range []int64{-1, 0, 0} {
		node := rec(coll.Elem, num(int64(i)), none(coll.Elem.(*types.RecordType).Fields[1].Type))
		if parent >= 0 {
			node.Fields[1] = &value.Ref{T: coll.Elem.(*types.RecordType).Fields[1].Type.(*types.OptionalType).Elem, Key: value.Key{I: parent, IsInt: true}}
		}
		node.Ident = &value.Identity{Coll: coll, Key: value.Key{I: int64(i), IsInt: true}}
		nodes.Elems = append(nodes.Elems, node)
	}
	roundTrip(t, rec(tree, nodes), tree)
}

// gtype draws random types and values of them for the round trip (WIRE.md §5).
type gtype struct {
	r     *rand.Rand
	enums []*types.EnumType
	n     int
}

func (g *gtype) name() string {
	g.n++
	return "t" + strconv.Itoa(g.n)
}

// scalar is a random scalar type and a function drawing its values.
func (g *gtype) scalar() (types.Type, func() value.Value) {
	sized := []types.Basic{types.IntType, types.Int8Type, types.Int16Type, types.Int32Type, types.UInt8Type, types.UInt16Type, types.UInt32Type, types.UInt64Type}
	switch g.r.IntN(7) {
	case 0:
		return types.BoolType, func() value.Value { return boolean(g.r.IntN(2) == 0) }
	case 1:
		b := sized[g.r.IntN(len(sized))]
		lo, hi, _ := b.Limits()
		return b, func() value.Value { return &value.Int{V: lo + int64(g.r.Uint64N(uint64(hi-lo))), T: b} }
	case 2:
		return types.FloatType, func() value.Value { return flt(g.float()) }
	case 3:
		return types.Float32Type, func() value.Value {
			return &value.Float{V: float64(float32(g.r.NormFloat64() * 1e6)), T: types.Float32Type}
		}
	case 4:
		return types.DurationType, func() value.Value { return dur(g.r.Int64N(2*types.DurationLimit) - types.DurationLimit) }
	case 5:
		e := g.enums[g.r.IntN(len(g.enums))]
		return e, func() value.Value { return member(e, g.r.IntN(len(e.Members))) }
	}
	return types.StringType, func() value.Value { return str((&gen{r: g.r}).string()) }
}

func (g *gtype) float() float64 {
	x := math.Float64frombits(g.r.Uint64())
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0.5
	}
	return x
}

// of is a random type of depth at most d, with a function drawing its values.
func (g *gtype) of(d int) (types.Type, func() value.Value) {
	if d == 0 {
		return g.scalar()
	}
	switch g.r.IntN(9) {
	case 5:
		return g.variant(d)
	case 6:
		return g.table(d)
	case 7:
		return g.keyedList(d)
	case 0:
		t, draw := g.of(d - 1)
		if t.Kind() == types.Optional {
			return t, draw
		}
		o := opt(t)
		return o, func() value.Value {
			if g.r.IntN(3) == 0 {
				return none(o)
			}
			return draw()
		}
	case 1:
		t, draw := g.of(d - 1)
		lt := listOf(t)
		return lt, func() value.Value {
			l := &value.List{T: lt}
			for range g.r.IntN(4) {
				l.Elems = append(l.Elems, draw())
			}
			return l
		}
	case 2:
		return g.mapOf(d)
	case 3:
		return g.record(d)
	}
	return g.scalar()
}

func (g *gtype) mapOf(d int) (types.Type, func() value.Value) {
	vt, draw := g.of(d - 1)
	e := g.enums[g.r.IntN(len(g.enums))]
	keys := []types.Type{types.StringType, types.IntType, e}
	mt := &types.MapType{Key: keys[g.r.IntN(len(keys))], Value: vt}
	return mt, func() value.Value {
		m := &value.Map{T: mt}
		seen := map[string]bool{}
		for range g.r.IntN(4) {
			var k value.Value
			switch mt.Key.Kind() {
			case types.String:
				k = str((&gen{r: g.r}).string())
			case types.Int:
				k = num(g.r.Int64N(2000) - 1000)
			default:
				k = member(e, g.r.IntN(len(e.Members)))
			}
			if text := k.CanonText(); !seen[text] {
				seen[text] = true
				m.Keys, m.Vals = append(m.Keys, k), append(m.Vals, draw())
			}
		}
		return m
	}
}

func (g *gtype) record(d int) (types.Type, func() value.Value) {
	rt := &types.RecordType{Pkg: "p", Name: g.name()}
	var draws []func() value.Value
	for i := range g.r.IntN(4) {
		ft, draw := g.of(d - 1)
		f := field("f"+strconv.Itoa(i), ft)
		if g.r.IntN(4) == 0 {
			f.WirePath = []string{"box", f.Name}
		}
		f.Index = i
		draw = g.annotate(f, draw)
		rt.Fields, draws = append(rt.Fields, f), append(draws, draw)
	}
	if g.r.IntN(3) == 0 {
		f, draw := g.special(len(rt.Fields), d)
		rt.Fields, draws = append(rt.Fields, f), append(draws, draw)
	}
	return rt, func() value.Value {
		r := rec(rt)
		for _, draw := range draws {
			r.Fields = append(r.Fields, draw())
		}
		return r
	}
}

// WIRE.md §5, IMPLEMENTATION-PLAN.md §7.7: decode(encode(v)) == v for random types and values.
func TestDecodeRoundTrip(t *testing.T) {
	codesEnum := codes(enum("p", "C", "a", "b", "c"), 1, 7, 42)
	codesEnum.WireCodes = true
	g := &gtype{r: rand.New(rand.NewPCG(3, 4)), enums: []*types.EnumType{tone, codesEnum, side}}
	for range 3000 {
		rt, draw := g.record(3)
		roundTrip(t, draw(), rt)
	}
}

// FuzzDecode: no input makes decoding panic; a failure always has a finding inside its file
// (or a dereference the host refused), and a success is a value.
func FuzzDecode(f *testing.F) {
	for _, seed := range []string{`{"dwID": "II", "szName": "N", "nHeal": 1, "dwCooldownMs": 1.5}`, `{"nodes": [{"id": 0, "parent": -1}]}`,
		`{"version": 2, "tasks": [{"eventType": "ECONOMY_DROP_ITEM", "filterParam": "", "targetPerPlayer": 1, "maxDurationMin": 1e2, "description": "d"}]}`,
		`{"id": "e", "type": "spawn_monster", "schedule": [{"day": "Mon"}], "worldId": -1}`, `[1, null, {"a": 1e999999}]`, `{"k0": "a", "v1": null}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		h := newHost()
		tree, _ := talentFixture()
		config, _ := heistia(h, newVocab(h))
		skill, _, _ := fpdemoSkill()
		for _, typ := range []types.Type{potionFixture(h), tree, config, eventFixture(h), skill, listOf(opt(types.Float32Type)),
			&types.MapType{Key: types.UInt8Type, Value: &types.TableType{Elem: status}}} {
			fuzzOne(t, h, src, typ)
		}
	})
}

func fuzzOne(t *testing.T, h *host, src string, typ types.Type) {
	fs := &source.FileSet{}
	file, err := fs.Add("f.json", "/p/f.json", []byte(src))
	if err != nil {
		return
	}
	bag := diag.NewBag(fs, "p")
	root, err := jsonsrc.Parse(file, bag)
	if err != nil {
		return
	}
	dec := wire.Decoder{Bag: bag, Pkg: "p", Host: h}
	misses := h.misses
	v, ok, err := dec.Decode(context.Background(), wire.Selection{Node: root}, typ)
	findings := bag.Findings()
	switch {
	case err != nil:
		t.Fatalf("%v: misuse %v", typ, err)
	case ok && (v == nil || len(findings) > 0):
		t.Fatalf("%v: ok with %v, %v", typ, v, findings)
	case !ok && len(findings) == 0 && h.misses == misses:
		t.Fatalf("%v: failed without a finding", typ)
	}
	for _, fd := range findings {
		if fd.Span.File != file.ID || fd.Span.Start > fd.Span.End || int(fd.Span.End) > len(file.Content) {
			t.Fatalf("%v: finding %s out of the file: %+v", typ, fd.Code, fd.Span)
		}
	}
}

// annotate gives a field a random @json form its type allows (WIRE.md §4.1).
func (g *gtype) annotate(f *types.Field, draw func() value.Value) func() value.Value {
	switch {
	case f.Type == types.DurationType:
		f.Unit = types.Unit(g.r.IntN(int(types.UnitD) + 1))
		per := f.Unit.Millis()
		return func() value.Value {
			return dur(per * (g.r.Int64N(2*types.DurationLimit/per) - types.DurationLimit/per))
		}
	case f.Type == types.BoolType:
		f.Enc = types.Enc(g.r.IntN(2))
	case f.Type.Kind() == types.Optional && g.r.IntN(2) == 0:
		if k := f.Type.Base().(*types.OptionalType).Elem.Base().Kind(); k != types.Map && k != types.Record && k != types.Table && k != types.Variant {
			f.NoneWire = []byte("{}")
		}
	}
	return draw
}

// special is a field with a whole-record form: bits, pairs or an inline variant.
func (g *gtype) special(i, d int) (*types.Field, func() value.Value) {
	switch g.r.IntN(3) {
	case 0:
		f := field("bits", listOf(flag))
		f.Enc, f.Index = types.EncBits, i
		return f, func() value.Value {
			l := list(flag)
			for m := range flag.Members {
				if g.r.IntN(2) == 0 {
					l.Elems = append(l.Elems, member(flag, m))
				}
			}
			return l
		}
	case 1:
		pair := record("p", g.name(), field("k", types.StringType), field("v", types.IntType))
		f := field("pairs", listOf(pair))
		f.Pairs, f.WirePath, f.Index = &types.Pairs{Keys: [2]string{"pk{i}", "pv{i}"}, Slots: 3}, nil, i
		return f, func() value.Value {
			l := list(pair)
			for n := range g.r.IntN(4) {
				l.Elems = append(l.Elems, rec(pair, str("a"), num(int64(n))))
			}
			return l
		}
	}
	vt, draw := g.variant(d)
	f := field("inline", vt)
	f.Inline, f.WirePath, f.Index = true, nil, i
	return f, draw
}

// variant is a variant of up to three cases, each with up to two fields.
func (g *gtype) variant(d int) (types.Type, func() value.Value) {
	var cases []*types.CaseType
	var draws [][]func() value.Value
	for c := range 1 + g.r.IntN(3) {
		ct := &types.CaseType{Name: "case" + strconv.Itoa(c)}
		var fd []func() value.Value
		for j := range g.r.IntN(3) {
			ft, draw := g.of(max(d-2, 0))
			ct.Fields, fd = append(ct.Fields, field("c"+strconv.Itoa(j), ft)), append(fd, draw)
		}
		cases, draws = append(cases, ct), append(draws, fd)
	}
	vt := variantOf("p", g.name(), []string{"kind", "type"}[g.r.IntN(2)], cases...)
	return vt, func() value.Value {
		c := g.r.IntN(len(cases))
		r := rec(cases[c])
		for _, draw := range draws[c] {
			r.Fields = append(r.Fields, draw())
		}
		return r
	}
}

// table is a nested table: entries k0, k1… with some retired (WIRE.md §5.7).
func (g *gtype) table(d int) (types.Type, func() value.Value) {
	et, draw := g.record(d - 1)
	tt := &types.TableType{Elem: et}
	return tt, func() value.Value {
		tv := &value.Table{T: tt}
		for n := range g.r.IntN(4) {
			e := draw().(*value.Record)
			e.Ident = &value.Identity{Key: value.Key{S: "k" + strconv.Itoa(n)}, Retired: g.r.IntN(3) == 0}
			tv.Entries = append(tv.Entries, e)
		}
		return tv
	}
}

// keyedList is a list keyed by an Int field, with refs into it from a field of its elements.
func (g *gtype) keyedList(d int) (types.Type, func() value.Value) {
	et, draw := g.record(d - 1)
	rt := et.(*types.RecordType)
	coll := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: g.name(), Elem: rt}
	id := field("id", types.IntType)
	id.Index = len(rt.Fields)
	link := field("link", &types.LitUnionType{Of: &types.RefType{Target: coll}, Literals: []string{"none"}})
	link.Index = id.Index + 1
	rt.Fields = append(rt.Fields, id, link)
	coll.KeyedBy = id
	lt := &types.ListType{Elem: rt, KeyedBy: id}
	return lt, func() value.Value {
		l := &value.List{T: lt}
		for n := range g.r.IntN(4) {
			e := draw().(*value.Record)
			var to value.Value = &value.Str{V: "none", T: link.Type}
			if n > 0 {
				to = &value.Ref{T: link.Type.(*types.LitUnionType).Of, Key: value.Key{I: int64(n - 1), IsInt: true}}
			}
			e.Fields = append(e.Fields, num(int64(n)), to)
			e.Ident = &value.Identity{Coll: coll, Key: value.Key{I: int64(n), IsInt: true}}
			l.Elems = append(l.Elems, e)
		}
		return l
	}
}
