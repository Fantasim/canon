package canon_test

import (
	"fmt"
	"strings"
	"testing"
)

// layerMapKeySrc is items, holding {s}, and z, whose map, dependent map and list keyed by a ref to
// an item each hold {s}.
const layerMapKeySrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord T {\n  it: ref items\n  n: Int = 0\n}\n\n" +
	"record Z {\n  weights: {ref items: Int}\n  dw: {k in items: Int}\n  ts: [T] keyed by it\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{s}\" }, { id: \"bow\" }]\n\n" +
	"let z: Z = { weights: { {s}: 1, bow: 2 }, dw: { {s}: 1, bow: 2 }, ts: [{ it: {s} }] }\n"

// API.md R7, E11, E22 (DECISIONS 316, 484): an amend path's key typed `ref items`, of a map, a
// dependent map or, as a selector, a list keyed by the ref, is a `layer` ref of the item it names,
// listed once and rewritten once in its form, active or not; the Undo gives the bytes back.
func TestRefsLayerRefTypedKeySegment(t *testing.T) {
	rows := []struct{ before, after string }{
		{"weights[{s}]: 4", "weights[y]: 4"},
		{"weights[\"{s}\"]: 4", "weights[\"y\"]: 4"},
		{"dw[{s}]: 4", "dw[y]: 4"},
		{"ts[items.{s}].n: 4", "ts[items.y].n: 4"},
	}
	want := fmt.Sprintf("layer %s:5, key %s:20, key %s:20, value %s:20", sharedLayer, srcMain, srcMain, srcMain)
	for _, active := range []bool{true, false} {
		var layers []string
		if active {
			layers = []string{"dev"}
		}
		for _, r := range rows {
			files := func(key, line string) map[string]string {
				layer := "package p\nlayer dev\n\namend z {\n  " + line + "\n}\n"
				fill := strings.NewReplacer("{s}", key)
				return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(layerMapKeySrc)), sharedLayer: formatted(t, sharedLayer, fill.Replace(layer))}
			}
			t.Run(fmt.Sprintf("active %v %s", active, r.before), func(t *testing.T) {
				sharedRow{l: sharedLevel{path: "p:items[shield]"}, layers: layers, before: files("shield", r.before), after: files("y", r.after), want: want, to: "y"}.run(t)
			})
		}
	}
}
