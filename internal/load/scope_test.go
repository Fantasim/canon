package load_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// scopeField is a field scope with unit u, encoding e and none marker none (nil for null).
func scopeField(u types.Unit, e types.Enc, none string) *types.Field {
	f := &types.Field{Name: "x", Unit: u, Enc: e}
	if none != "" {
		f.NoneWire = []byte(none)
	}
	return f
}

// markType is `[Mark]`, Mark an enum with @codes a = 1, b = 2, c = 4 (WIRE.md §5.3).
func markType() *types.ListType {
	mark := &types.EnumType{Pkg: "p", Name: "Mark", Codes: &types.UInt8Type}
	for i, name := range []string{"a", "b", "c"} {
		mark.Members = append(mark.Members, &types.Member{Name: name, Wire: name, Index: i, Code: 1 << i, HasCode: true})
	}
	return &types.ListType{Elem: mark}
}

// waitType is a record whose Duration field has no unit of its own: ms on the wire.
func waitType() *types.RecordType {
	t := &types.Field{Name: "t", Type: types.DurationType, Wire: "t", WirePath: []string{"t"}}
	return &types.RecordType{Pkg: "p", Name: "Wait", Fields: []*types.Field{t}}
}

// WIRE.md §4.1, §6.1, §6.5, §6.6, DECISIONS 268: a load decodes in Request.Field's scope.
func TestLoadFieldScope(t *testing.T) {
	unitS := scopeField(types.UnitS, types.EncPlain, "")
	optDur := &types.OptionalType{Elem: types.DurationType}
	defines := &types.TableType{Elem: types.DefineType}
	for _, c := range []struct {
		name  string
		files map[string]string
		expr  *syntax.LoadExpr
		typ   types.Type
		f     *types.Field
		want  string
	}{
		{"no field", map[string]string{"d.json": `5`}, bareExpr("d.json"), types.DurationType, nil, "5ms"},
		{"unit", map[string]string{"d.json": `5`}, bareExpr("d.json"), types.DurationType, unitS, "5s"},
		{"int", map[string]string{"b.json": `1`}, bareExpr("b.json"), types.BoolType, scopeField(types.UnitMs, types.EncInt, ""), "true"},
		{"none marker", map[string]string{"o.json": `0`}, bareExpr("o.json"), optDur, scopeField(types.UnitS, types.EncPlain, "0"), "none"},
		{"not the marker", map[string]string{"o.json": `5`}, bareExpr("o.json"), optDur, scopeField(types.UnitS, types.EncPlain, "0"), "5s"},
		{"bits", map[string]string{"m.json": `5`}, bareExpr("m.json"), markType(), scopeField(types.UnitMs, types.EncBits, ""), "[a, c]"},
		{"unit, list", map[string]string{"l.json": `[5, 7]`}, bareExpr("l.json"), &types.ListType{Elem: types.DurationType}, unitS, "[5s, 7s]"},
		{"load.dir unit", map[string]string{"d/a.json": `5`, "d/b.json": `7`}, dirExpr("d/*.json"), &types.ListType{Elem: types.DurationType}, unitS, "[5s, 7s]"},
		{"load.dir int", map[string]string{"d/a.json": `1`, "d/b.json": `0`}, dirExpr("d/*.json"), &types.ListType{Elem: types.BoolType}, scopeField(types.UnitMs, types.EncInt, ""), "[true, false]"},
		{"load.dir rows", map[string]string{"d/a.json": `{"t": 5}`}, dirExpr("d/*.json"), &types.TableType{Elem: waitType()}, unitS, "{a: Wait{t: 5ms}}"},
		{"load.csv rows", map[string]string{"w.csv": "t\n5\n"}, withHeader(csvExpr("w.csv")), &types.ListType{Elem: waitType()}, unitS, "[Wait{t: 5ms}]"},
		{"load.defines", map[string]string{"d.h": "#define A 5\n"}, definesExpr("d.h", ""), defines, unitS, "{A: Define{value: 5}}"},
	} {
		l, req := loaderFor(t, c.files)
		req.Field = c.f
		v, ok, err := l.Load(context.Background(), req, c.expr, c.typ)
		if err != nil || !ok || v.CanonText() != c.want {
			t.Errorf("%s: %v ok=%v err=%v findings=%+v, want %s", c.name, v, ok, err, req.Bag.Findings(), c.want)
		}
	}
}
