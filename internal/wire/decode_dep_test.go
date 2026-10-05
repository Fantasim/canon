package wire_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// optionalDriver is a Task applying Param to its optional `eventType`, for an optional and a
// required field, and a Probe applying it to `column.values`, an optional field read through a ref.
func optionalDriver(h *host) (task, probe *types.RecordType) {
	v := newVocab(h)
	eventType := field("eventType", opt(&types.RefType{Target: v.eventTypes}))
	byEvent := []*types.Arg{{Source: types.ArgField, Path: []*types.Field{eventType}}}
	task = record("resource.heistia", "Task", eventType,
		field("filterParam", opt(&types.TypeAppType{Fn: v.param, Args: byEvent})),
		field("target", &types.TypeAppType{Fn: v.param, Args: byEvent}))
	values := field("values", opt(&types.RefType{Target: v.eventTypes}))
	col := record("resource.heistia", "Column", field("name", types.StringType), values)
	columns := &types.Collection{Kind: types.CollLet, Pkg: "resource.heistia", Name: "columns", Elem: col}
	h.entries[columns] = []*value.Record{
		entry(columns, "C0", false, rec(col, str("c0"), none(values.Type))),
		entry(columns, "C1", false, rec(col, str("c1"), ref(v.eventTypes, "COMBAT_GAME_MODE_START"))),
	}
	column := field("column", &types.RefType{Target: columns})
	byColumn := []*types.Arg{{Source: types.ArgField, Path: []*types.Field{column, values}}}
	probe = record("resource.heistia", "Probe", column, field("filter", opt(&types.TypeAppType{Fn: v.param, Args: byColumn})))
	return task, probe
}

// TYPES.md §11.6, WIRE.md §5.9, DECISIONS 307: a none argument selects Never, a set one its branch.
func TestDecodeOptionalArgument(t *testing.T) {
	h := newHost()
	task, probe := optionalDriver(h)
	cases := []struct {
		json string
		typ  *types.RecordType
		want string
	}{
		{`{"target": "t"}`, task, "none"},
		{`{"eventType": null, "target": "t"}`, task, "none"},
		{`{"eventType": null, "filterParam": null, "target": "t"}`, task, "none"},
		{`{"filterParam": "MI_AIBATT1", "target": "t"}`, task, "symbol MI_AIBATT1"},
		{`{"eventType": null, "filterParam": 5, "target": "t"}`, task, "symbol 5"},
		{`{"eventType": null}`, task, at(diag.E3302, "1:1", "")},
		{`{"eventType": null, "target": null}`, task, at(diag.E3315, "1:31", "/target")},
		{`{"eventType": "ECONOMY_DROP_ITEM", "filterParam": "II_X", "target": "II_Y"}`, task, "ref II_X"},
		{`{"eventType": "COMBAT_GAME_MODE_START", "filterParam": "pvp", "target": "pve"}`, task, "string pvp"},
		{`{"eventType": "ECONOMY_DROP_ITEM", "filterParam": 5, "target": "II_Y"}`, task, at(diag.E7110, "1:51", "/filterParam")},
		{`{"column": "C0"}`, probe, "none"},
		{`{"column": "C0", "filter": null}`, probe, "none"},
		{`{"column": "C0", "filter": "pvp"}`, probe, "symbol pvp"},
		{`{"column": "C1", "filter": "pvp"}`, probe, "string pvp"},
	}
	for _, c := range cases {
		if got := branchText(decodeJSON(t, wire.Decoder{Host: h}, c.json, c.typ)); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
	if h.misses != 0 {
		t.Errorf("a none argument is no entry to dereference: %d misses", h.misses)
	}
}

// TYPES.md §11.6, DECISIONS 307, 175: a required field on a none argument keeps a symbol for E3801.
func TestDecodeOptionalArgumentRequired(t *testing.T) {
	h := newHost()
	task, _ := optionalDriver(h)
	got := decodeJSON(t, wire.Decoder{Host: h}, `{"target": "MI_AIBATT1"}`, task)
	if !got.ok {
		t.Fatalf("decode: %s", got.text())
	}
	if s, ok := got.v.(*value.Record).Fields[2].(*value.Symbol); !ok || s.Name != "MI_AIBATT1" {
		t.Errorf("target: %s, want the symbol MI_AIBATT1", got.v.(*value.Record).Fields[2].CanonText())
	}
}

// optionalContainers is a Rates applying Param to its optional `eventType` as a map key and as a
// list element.
func optionalContainers(h *host) *types.RecordType {
	v := newVocab(h)
	eventType := field("eventType", opt(&types.RefType{Target: v.eventTypes}))
	app := &types.TypeAppType{Fn: v.param, Args: []*types.Arg{{Source: types.ArgField, Path: []*types.Field{eventType}}}}
	return record("resource.heistia", "Rates", eventType,
		field("rates", opt(&types.MapType{Key: app, Value: types.IntType})), field("targets", opt(listOf(app))))
}

// leafText names what one key or element decoded as.
func leafText(v value.Value) string {
	switch x := v.(type) {
	case *value.Symbol:
		return "symbol " + x.Name
	case *value.Ref:
		return "ref " + x.Key.Text()
	case *value.Str:
		return "string " + x.V
	}
	return v.CanonText()
}

// leaves names the keys of rates, then the elements of targets.
func leaves(d decoded) string {
	if !d.ok {
		return d.text()
	}
	var out []string
	r := d.v.(*value.Record)
	if m, ok := r.Fields[1].(*value.Map); ok {
		for _, k := range m.Keys {
			out = append(out, leafText(k))
		}
	}
	if l, ok := r.Fields[2].(*value.List); ok {
		for _, e := range l.Elems {
			out = append(out, leafText(e))
		}
	}
	return strings.Join(out, ", ")
}

// TYPES.md §11.6, WIRE.md §5.4, §5.9, DECISIONS 307: a none argument gives Never keys and elements.
func TestDecodeOptionalArgumentContainers(t *testing.T) {
	h := newHost()
	rates := optionalContainers(h)
	for _, c := range []struct{ json, want string }{
		{`{"rates": {"MI_A": 1}, "targets": ["x", 5]}`, "symbol MI_A, symbol x, symbol 5"},
		{`{"eventType": null, "rates": {}, "targets": []}`, ""},
		{`{"eventType": null, "targets": [null]}`, at(diag.E3315, "1:33", "/targets/0")},
		{`{"eventType": "ECONOMY_DROP_ITEM", "rates": {"II_A": 1}, "targets": ["II_B"]}`, "ref II_A, ref II_B"},
		{`{"eventType": "COMBAT_GAME_MODE_START", "rates": {"pvp": 1}, "targets": ["pve"]}`, "string pvp, string pve"},
	} {
		if got := leaves(decodeJSON(t, wire.Decoder{Host: h}, c.json, rates)); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
}
