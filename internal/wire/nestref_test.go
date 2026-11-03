package wire_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// keyedBy is a let keyed list of records whose one field, key, has type kt.
func keyedBy(name string, kt types.Type) *types.Collection {
	row := record("p", name+"Row", field("key", kt))
	return &types.Collection{Kind: types.CollLet, Pkg: "p", Name: name, Elem: row, KeyedBy: row.Fields[0]}
}

// refChain is a ref into base (depth 1), or into depth-1 lists each keyed by a ref into the last.
func refChain(base *types.Collection, depth int) *types.RefType {
	coll := base
	for range depth - 1 {
		coll = keyedBy(coll.Name+"x", &types.RefType{Target: coll})
	}
	return &types.RefType{Target: coll}
}

// nestHolder is a record holding a ref of rt as a field, a list element, a map key, a case field
// of a variant (union arm) and, with spot, a literal union's ref (string wire forms only, E3002).
func nestHolder(rt *types.RefType, spot bool) (*types.RecordType, func(key value.Key) value.Value) {
	arm := &types.CaseType{Name: "pick", Fields: []*types.Field{field("to", rt)}}
	vt := variantOf("p", "Arm", "kind", arm)
	mt := &types.MapType{Key: rt, Value: types.IntType}
	fields := []*types.Field{field("one", rt), field("many", listOf(rt)), field("byKey", mt)}
	if spot {
		fields = append(fields, field("spot", &types.LitUnionType{Of: rt, Literals: []string{"none"}}))
	}
	holder := record("p", "Holder", append(fields, field("arm", vt))...)
	return holder, func(key value.Key) value.Value {
		r := func() *value.Ref { return &value.Ref{T: rt, Key: key} }
		m := &value.Map{T: mt, Keys: []value.Value{r()}, Vals: []value.Value{num(1)}}
		v := rec(holder, r(), &value.List{T: listOf(rt), Elems: []value.Value{r()}}, m)
		if spot {
			v = rec(holder, append(v.Fields, r())...)
		}
		return rec(holder, append(v.Fields, rec(arm, r()))...)
	}
}

// holderJSON is nestHolder's document, whitespace removed, with every ref written as want.
func holderJSON(want string, spot bool) string {
	s := `{"$schema":"p.Holder@00000000","value":{"one":` + want + `,"many":[` + want + `],"byKey":{"` +
		strings.Trim(want, `"`) + `":1},`
	if spot {
		s += `"spot":` + want + `,`
	}
	return s + `"arm":{"kind":"pick","to":` + want + `}}}`
}

// WIRE.md §5.9, DECISIONS 387/432: a nested ref key is its leaf key's wire, and reads back.
func TestNestedRefKeyWire(t *testing.T) {
	tone := enumWires("p", "Tone", "first", "first", "second", "z2", "third", "third")
	coded := codes(enumWires("p", "Coded", "low", "low", "high", "high"), 3, 7)
	coded.WireCodes = true
	byTone, byCode := keyedBy("l0", tone), keyedBy("l0", coded)
	byStr, byInt := keyedBy("l0", types.StringType), keyedBy("l0", types.IntType)
	table := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "l0", Elem: record("p", "Row")}
	second, seven := value.Key{S: "second"}, value.Key{I: 7, IsInt: true}
	cases := []struct {
		rule  string
		base  *types.Collection
		depth int
		key   value.Key
		want  string
		spot  bool
	}{
		{"§5.9 enum leaf, one level", byTone, 1, second, `"z2"`, true},
		{"§5.9 enum leaf, two levels", byTone, 2, second, `"z2"`, true},
		{"§5.9 enum leaf, three levels", byTone, 3, second, `"z2"`, true},
		{"§5.9 @json(codes) leaf, two levels", byCode, 2, value.Key{S: "high"}, `7`, false},
		{"§5.9 @json(codes) leaf, three levels", byCode, 3, value.Key{S: "high"}, `7`, false},
		{"§5.9 String leaf, three levels", byStr, 3, second, `"second"`, true},
		{"§5.9 Int leaf, three levels", byInt, 3, seven, `7`, false},
		{"§5.9 table leaf, one level", table, 1, second, `"second"`, true},
		{"§5.9 table leaf, three levels", table, 3, second, `"second"`, true},
	}
	for _, c := range cases {
		holder, draw := nestHolder(refChain(c.base, c.depth), c.spot)
		v := draw(c.key)
		got := strings.Join(strings.Fields(encode(t, &wire.Document{Schema: "p.Holder@00000000", Kind: types.Record, V: v})), "")
		if want := holderJSON(c.want, c.spot); got != want {
			t.Errorf("%s:\n got %s\nwant %s", c.rule, got, want)
		}
		roundTrip(t, v, holder)
	}
}

// TYPES.md §9.1, DECISIONS 316: a self-keyed list (E3012) stops the key walk; the key is text.
func TestNestedRefKeyCycle(t *testing.T) {
	self := keyedBy("loop", nil)
	self.KeyedBy.Type = &types.RefType{Target: self}
	r := &value.Ref{T: self.KeyedBy.Type, Key: value.Key{S: "k"}}
	got := strings.Join(strings.Fields(encode(t, &wire.Document{Schema: "p.Loop@00000000", Kind: types.Ref, V: r})), "")
	if !strings.Contains(got, `"value":"k"`) {
		t.Errorf("cyclic ref key: %s", got)
	}
}
