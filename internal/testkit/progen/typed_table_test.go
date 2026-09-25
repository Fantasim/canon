package progen_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	envValue = "value" // WIRE.md §8.1: a value's JSON envelope key
	envRows  = "rows"  // WIRE.md §5.1: a table's
	rowID    = "$id"
)

// tableKeys are a generated table's two entry keys, in declaration order.
var tableKeys = []string{"keyA", "keyB"}

// tableRoot is one `table` root value (WIRE.md §5.1): its entries' literal text and field values.
type tableRoot struct {
	name, recName string
	entryText     map[string]string
	entries       map[string]map[string]any
}

// refRoot is one `ref` root value into a tableRoot (WIRE.md §5.11).
type refRoot struct {
	name, recName, key string
}

// genTable is a table of rec's type, its two entries built like a record root's (genRecordValue),
// so a field default is sometimes omitted there too.
func genTable(r *progen.Rand, mo *typedModel, rec recordDef) tableRoot {
	tr := tableRoot{name: "items", recName: rec.name, entryText: map[string]string{}, entries: map[string]map[string]any{}}
	for _, k := range tableKeys {
		text, vals := genRecordValue(r, mo, rec, false)
		tr.entryText[k], tr.entries[k] = text, vals
	}
	return tr
}

// renderTable writes a table root's `let` declaration.
func renderTable(b *strings.Builder, tr tableRoot) {
	fmt.Fprintf(b, "let %s: table %s = {\n", tr.name, tr.recName)
	for _, k := range tableKeys {
		fmt.Fprintf(b, "  %s %s\n", k, tr.entryText[k])
	}
	b.WriteString("}\n")
}

// renderRef writes a ref root's `let` declaration.
func renderRef(b *strings.Builder, rr refRoot) {
	fmt.Fprintf(b, "let %s: ref %s = %s\n", rr.name, rr.recName, rr.key)
}

// expectations is expectFile: each known value's JSON envelope (WIRE.md §8.1, §5.1); no probe.
func expectations(tc typedCase) []byte {
	want := map[string]map[string]any{}
	for _, root := range tc.roots {
		want[root.name] = map[string]any{envValue: root.vals}
	}
	rows := make([]map[string]any, 0, len(tableKeys))
	for _, k := range tableKeys {
		row := maps.Clone(tc.table.entries[k])
		row[rowID] = k
		rows = append(rows, row)
	}
	want[tc.table.name] = map[string]any{envRows: rows}
	want[tc.ref.name] = map[string]any{envValue: tc.ref.key}
	data, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		panic(err) // maps of JSON values always marshal
	}
	return append(data, '\n')
}

// checkExpected is "" when every value expect names has its JSON file, holding what expect says.
func checkExpected(byPath map[string][]byte, expect []byte) verdict {
	var want map[string]map[string]any
	if err := json.Unmarshal(expect, &want); err != nil {
		return harnessVerdict(err)
	}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		data, ok := byPath[typedDataOut+name+".json"]
		if !ok {
			return verdict{Kind: kindMissingJSON, Sig: kindMissingJSON + " " + generic(name), Text: "no JSON output for " + name}
		}
		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			return verdict{Kind: kindBadJSON, Sig: kindBadJSON + " " + generic(name), Text: name + ": " + err.Error()}
		}
		for _, key := range slices.Sorted(maps.Keys(want[name])) {
			if !reflect.DeepEqual(got[key], want[name][key]) {
				sig := kindJSONMismatch + " " + generic(name) + " " + key
				return verdict{Kind: kindJSONMismatch, Sig: sig, Text: fmt.Sprintf("%s %s: got %v, want %v", name, key, got[key], want[name][key])}
			}
		}
	}
	return verdict{}
}
