package canon_test

import (
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// layerKeyHead is the layer file's text before its lines.
const layerKeyHead = "package p\nlayer dev\n\namend aa {\n"

// layerKeyLevel is aa whose entries first and second each hold, in oo, an entry {k} keyed x
// (level 1), or an entry q of x's inn (level 2), through a plain or an optional field: path is
// the renamed entry's, and pre the line path reaching the instance holding it, %s first or second.
type layerKeyLevel struct {
	name, src, path, pre string
}

// layerKeyLevels are a level-1 and a level-2 entry, each through a plain and an optional field.
func layerKeyLevels() []layerKeyLevel {
	var out []layerKeyLevel
	for _, typ := range []struct{ name, list, inn string }{
		{"plain", "[S] keyed by k", "[T] keyed by k"},
		{"optional", "([S] keyed by k)?", "([T] keyed by k)?"},
	} {
		out = append(out, layerKeyLevel{"level 1 " + typ.name, "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n  v: Int = 0\n}\n\n" +
			"record A {\n  id: String\n  oo: " + typ.list + "\n}\n\n" +
			"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }] },\n" +
			"  { id: \"second\", oo: [{ k: \"{k}\" }] },\n]\n", "p:aa[second].oo[x]", "%s.oo"},
			layerKeyLevel{"level 2 " + typ.name, "package p\n\nrecord T {\n  k: String\n  w: Int = 0\n  v: Int = 0\n}\n\n" +
				"record S {\n  k: String\n  inn: " + typ.inn + "\n}\n\n" +
				"record A {\n  id: String\n  oo: " + typ.list + "\n}\n\n" +
				"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\", inn: [{ k: \"q\" }] }] },\n" +
				"  { id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"{k}\" }] }] },\n]\n", "p:aa[second].oo[x].inn[q]", "%s.oo.x.inn"})
	}
	return out
}

// old is the key l renames.
func (l layerKeyLevel) old() string {
	return l.path[strings.LastIndex(l.path, "[")+1 : len(l.path)-1]
}

// API.md R7, R11, E11, E22 (DECISIONS 316, 470, 483): an amend path's `.x` or `["x"]` naming second's
// entry is one `layer` ref, active or not, rewritten once (`["a b"]` for a non-word); first's is none.
func TestRefsLayerKeySegment(t *testing.T) {
	type row struct {
		active bool
		lines  []string
		to     string
	}
	var rows []row
	for _, lines := range [][]string{{"first"}, {"second"}, {"first", "second"}, {"second", "second[]"}, {"first[]", "second[]"}} {
		for _, to := range []string{"y", "a b"} {
			rows = append(rows, row{true, lines, to}, row{false, lines, to})
		}
	}
	for _, l := range layerKeyLevels() {
		for _, r := range rows {
			name := fmt.Sprintf("%s active %v %s to %q", l.name, r.active, strings.Join(r.lines, "+"), r.to)
			t.Run(name, func(t *testing.T) { layerKeyCase(t, l, r.active, r.lines, r.to) })
		}
	}
}

// formatted is src in the canonical layout, which an edit requires of the files it writes.
func formatted(t *testing.T, name, src string) string {
	t.Helper()
	out, err := canon.Format(name, []byte(src))
	if err != nil {
		t.Fatalf("the fixture %s does not format: %v", name, err)
	}
	return string(out)
}

// layerKeySeg is key as an amend segment: `.k` for a word when dotted, else `["k"]` (API.md E11).
func layerKeySeg(key string, dotted bool) string {
	if dotted && !strings.Contains(key, " ") {
		return "." + key
	}
	return "[\"" + key + "\"]"
}

// layerKeyLine is one amend line of a layerKeyCase: the path to the keyed list, whether it is
// through second's instance and writes its key dotted, and the field it sets.
type layerKeyLine struct {
	pre            string
	second, dotted bool
	field          string
}

// layerKeyCase is one row of TestRefsLayerKeySegment: each of lines amends w (dotted, `.k`) or v
// (bracketed, `["k"]`, the name ending in []) of the entry keyed by the old key in first's or
// second's instance; the rename is to to.
func layerKeyCase(t *testing.T, l layerKeyLevel, active bool, lines []string, to string) {
	var parts []layerKeyLine
	var want []string // R8: the layer file sorts before p's
	for i, who := range lines {
		inst, bracket := strings.CutSuffix(who, "[]")
		field := ".w: 1"
		if bracket {
			field = ".v: 2"
		}
		parts = append(parts, layerKeyLine{fmt.Sprintf(l.pre, inst), inst == "second", !bracket, field})
		if inst == "second" {
			want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, strings.Count(layerKeyHead, "\n")+i+1))
		}
	}
	files := func(key string, bracketed bool) map[string]string { // bracketed: second's lines keep `["k"]`
		layer := layerKeyHead
		for _, p := range parts {
			k, dotted := l.old(), p.dotted
			if p.second {
				k, dotted = key, dotted && !bracketed
			}
			layer += "  " + p.pre + layerKeySeg(k, dotted) + p.field + "\n"
		}
		src := strings.ReplaceAll(l.src, "{k}", key)
		return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer+"}\n")}
	}
	var layers []string
	if active {
		layers = []string{"dev"}
	}
	row := sharedRow{l: sharedLevel{path: l.path}, layers: layers, before: files(l.old(), false), after: files(to, false), want: strings.Join(want, ", "), to: to}
	if strings.Contains(to, " ") {
		row.undone = files(l.old(), true) // the renamed `["a b"]` is renamed back in its form (E11), equal as a value (E22)
	}
	row.run(t)
}

// layerCopySrc is aa, whose second's oo holds an entry keyed k, and bb, a spread copy of second
// that keeps its entries (DECISIONS 410); a line on bb names second's entry through it.
func layerCopySrc(k string) string {
	return "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\n" +
		"record A {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }] },\n  { id: \"second\", oo: [{ k: \"" + k + "\" }] },\n]\n\n" +
		"let bb: A = { ...aa.second, id: \"third\" }\n"
}

// API.md R7, E11, E22 (DECISIONS 410, 473): a key segment through a spread copy names the source's
// entry, a `layer` ref rewritten once, none of first's. Inactive only until EVAL-LAYERSPREAD: an
// active amend re-owns bb's copied entries, against DECISIONS 410.
func TestRefsLayerKeySegmentThroughACopy(t *testing.T) {
	layer := func(k string) string {
		return "package p\nlayer dev\n\namend bb {\n  oo." + k + ".w: 1\n}\n"
	}
	files := func(k string) map[string]string {
		return map[string]string{srcMain: formatted(t, srcMain, layerCopySrc(k)), sharedLayer: formatted(t, sharedLayer, layer(k))}
	}
	want := fmt.Sprintf("layer %s:5", sharedLayer)
	sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, before: files("x"), after: files("y"), want: want}.run(t)
	sharedRow{l: sharedLevel{path: "p:aa[first].oo[x]"}, before: files("x"), after: map[string]string{
		srcMain:     formatted(t, srcMain, strings.Replace(layerCopySrc("x"), "first\", oo: [{ k: \"x\"", "first\", oo: [{ k: \"y\"", 1)),
		sharedLayer: formatted(t, sharedLayer, layer("x")),
	}}.run(t)
}

// layerRefKeySrc is mobs and zs, whose zones' spawns are keyed by a ref to a mob: both spawn mob k,
// written mob.
func layerRefKeySrc(k, mob string) string {
	return "package p\n\nrecord M {\n  id: String\n}\n\nrecord Sp {\n  mob: ref mobs\n  n: Int = 0\n}\n\n" +
		"record Z {\n  id: String\n  spawns: [Sp] keyed by mob\n}\n\n" +
		"let mobs: [M] keyed by id = [{ id: \"" + k + "\" }, { id: \"bear\" }]\n\n" +
		"let zs: [Z] keyed by id = [{ id: \"z1\", spawns: [{ mob: " + mob + " }] }, { id: \"z2\", spawns: [{ mob: " + mob + " }] }]\n"
}

// layerRefKeyLayer amends z1's spawn of a mob dotted and z2's bracketed.
func layerRefKeyLayer(dotted, bracketed string) string {
	return "package p\nlayer dev\n\namend zs {\n  z1.spawns" + dotted + ".n: 3\n  z2.spawns" + bracketed + ".n: 4\n}\n"
}

// API.md R7, E11, E22 (DECISIONS 310, 316): a key typed `ref mobs` in an amend path, `.wolf` as
// `[wolf]`, is a `layer` ref of mobs[wolf], rewritten once beside the cascade moving each spawn.
func TestRefsLayerRefKeySegment(t *testing.T) {
	want := fmt.Sprintf("layer %s:5, layer %s:6, value %s:20, value %s:21", sharedLayer, sharedLayer, srcMain, srcMain)
	rows := []struct{ to, mob, dotted, bracketed, undoMob, undoDotted, undoBracketed string }{
		{"timber", "timber", ".timber", "[timber]", "wolf", ".wolf", "[wolf]"},
		{"a b", `"a b"`, `["a b"]`, `["a b"]`, `"wolf"`, `["wolf"]`, `["wolf"]`}, // E22: values, each token in its form
	}
	for _, active := range []bool{true, false} {
		var layers []string
		if active {
			layers = []string{"dev"}
		}
		for _, r := range rows {
			t.Run(fmt.Sprintf("active %v to %q", active, r.to), func(t *testing.T) {
				files := func(k, mob, dotted, bracketed string) map[string]string {
					return map[string]string{srcMain: formatted(t, srcMain, layerRefKeySrc(k, mob)), sharedLayer: formatted(t, sharedLayer, layerRefKeyLayer(dotted, bracketed))}
				}
				sharedRow{l: sharedLevel{path: "p:mobs[wolf]"}, layers: layers, before: files("wolf", "wolf", ".wolf", "[wolf]"),
					after: files(r.to, r.mob, r.dotted, r.bracketed), undone: files("wolf", r.undoMob, r.undoDotted, r.undoBracketed), want: want, to: r.to}.run(t)
			})
		}
	}
}
