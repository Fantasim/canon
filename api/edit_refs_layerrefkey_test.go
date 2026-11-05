package canon_test

import (
	"fmt"
	"strings"
	"testing"
)

// layerRefKeysSrc is mobs and beasts, both holding a wolf, and zs, whose spawns are keyed by a ref to
// a mob and bs by a ref to a beast; {mw}, {mb} and {bw} are mobs' wolf and bear and beasts' wolf.
const layerRefKeysSrc = "package p\n\nrecord M {\n  id: String\n}\n\nrecord Sp {\n  mob: ref mobs\n  n: Int = 0\n}\n\n" +
	"record Bp {\n  b: ref beasts\n  n: Int = 0\n}\n\nrecord Z {\n  id: String\n  spawns: [Sp] keyed by mob\n  bs: [Bp] keyed by b\n}\n\n" +
	"let mobs: [M] keyed by id = [{ id: \"{mw}\" }, { id: \"{mb}\" }]\n\nlet beasts: [M] keyed by id = [{ id: \"{bw}\" }]\n\n" +
	"let zs: [Z] keyed by id = [\n  { id: \"z1\", spawns: [{ mob: {mw} }, { mob: {mb} }], bs: [{ b: {bw} }] },\n  { id: \"z2\", spawns: [{ mob: {mb} }], bs: [] },\n]\n"

// layerRefKeysLayer amends spawns of mobs' bear, dotted and bracketed, then beasts' wolf and mobs' wolf.
const layerRefKeysLayer = "package p\nlayer dev\n\namend zs {\n  z1.spawns.{mb}.n: 1\n  z2.spawns[{mb}].n: 3\n  z1.bs.{bw}.n: 4\n  z1.spawns.{mw}.n: 6\n}\n"

// layerRefKeysRow renames the entry at path, marked mark in the fixtures, from old to to: want are
// the layer file lines listed, values the source lines of the value refs.
type layerRefKeysRow struct {
	path, mark, old, to string
	want, values        []int
}

// API.md R7, E11, E22 (DECISIONS 316, 483): a ref key segment names only the entry its key's
// collection holds: renaming mobs' wolf, mobs' bear or beasts' wolf lists and rewrites its own
// lines once, never another mob's nor a same-keyed entry of another collection.
func TestRefsLayerRefKeySegmentOthers(t *testing.T) {
	rows := []layerRefKeysRow{
		{"p:mobs[wolf]", "{mw}", "wolf", "timber", []int{8}, []int{28}}, // 28: z1's line, 29: z2's
		{"p:mobs[bear]", "{mb}", "bear", "grizzly", []int{5, 6}, []int{28, 29}},
		{"p:beasts[wolf]", "{bw}", "wolf", "fox", []int{7}, []int{28}},
	}
	for _, active := range []bool{true, false} {
		var layers []string
		if active {
			layers = []string{"dev"}
		}
		for _, r := range rows {
			t.Run(fmt.Sprintf("active %v %s", active, r.path), func(t *testing.T) { r.run(t, layers) })
		}
	}
}

// run is one row of TestRefsLayerRefKeySegmentOthers with layers active.
func (r layerRefKeysRow) run(t *testing.T, layers []string) {
	files := func(key string) map[string]string {
		marks := map[string]string{"{mw}": "wolf", "{mb}": "bear", "{bw}": "wolf"}
		marks[r.mark] = key
		fill := strings.NewReplacer("{mw}", marks["{mw}"], "{mb}", marks["{mb}"], "{bw}", marks["{bw}"])
		return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(layerRefKeysSrc)), sharedLayer: formatted(t, sharedLayer, fill.Replace(layerRefKeysLayer))}
	}
	var want []string
	for _, n := range r.want {
		want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, n))
	}
	for _, n := range r.values {
		want = append(want, fmt.Sprintf("value %s:%d", srcMain, n))
	}
	sharedRow{l: sharedLevel{path: r.path}, layers: layers, before: files(r.old), after: files(r.to), want: strings.Join(want, ", "), to: r.to}.run(t)
}
