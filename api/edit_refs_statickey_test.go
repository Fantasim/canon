package canon_test

import (
	"fmt"
	"strings"
	"testing"
)

// staticShadowSrc is items, a table whose keys are known statically, {i} and axe, a let shield
// naming item axe, and many, whose one holds elements keyed {o} and axe; r1 reads `ts[{r}]`.
const staticShadowSrc = "package p\n\nrecord I {\n  v: Int = 0\n}\n\nrecord T {\n  k: ref items\n  n: Int = 0\n}\n\n" +
	"record O {\n  id: String\n  ts: [T] keyed by k\n}\n\n" +
	"let items: table I = {\n  {i} {}\n  axe {}\n}\n\n" +
	"let shield: ref items = axe\n\n" +
	"let many: [O] keyed by id = [{ id: \"one\", ts: [{ k: {o}, n: 1 }, { k: axe, n: 7 }] }]\n\n" +
	"let r1: Int = many.one.ts[{r}].n\n"

// API.md R7, E11, E22; TYPES.md §4.1 step 1; DECISIONS 490.
func TestRefsStaticKeyBeforeScope(t *testing.T) {
	for _, active := range []bool{true, false} {
		var layers []string
		want := map[string]string{"p:r1": "1"}
		if active {
			layers, want["p:r1"] = []string{"dev"}, "5"
		}
		t.Run(fmt.Sprintf("active %v", active), func(t *testing.T) {
			files := func(key string) map[string]string {
				fill := strings.NewReplacer("{i}", key, "{o}", key, "{r}", key)
				layer := "package p\nlayer dev\n\namend many {\n  one.ts[" + key + "].n: 5\n}\n"
				return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(staticShadowSrc)), sharedLayer: formatted(t, sharedLayer, layer)}
			}
			before := files("shield")
			refs := fmt.Sprintf("layer %s:5, value %s:%d, code %s:%d", sharedLayer, srcMain, lineOf(before[srcMain], "let many:"),
				srcMain, lineOf(before[srcMain], "let r1:"))
			p, _ := sharedOpen(t, before, layers)
			checkValues(t, p, "TYPES.md §4.1 step 1", want)
			sharedRow{l: sharedLevel{path: "p:items[shield]"}, layers: layers, before: before, after: files("spear"), want: refs, to: "spear"}.run(t)
			p, _ = sharedOpen(t, files("spear"), layers)
			checkValues(t, p, "TYPES.md §4.1 step 1", want)
		})
	}
}
