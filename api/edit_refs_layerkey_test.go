package canon_test

import (
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// layerKeyHead is the layer file's text before its lines.
const layerKeyHead = "package p\nlayer dev\n\namend aa {\n"

// layerKeyLevel is aa whose first and second each hold, in oo, an entry keyed x (level 1) or an entry
// q of x's inn (level 2), plain or optional, keyed {f} in first and {k} in second: path is second's
// entry, and pre the line path reaching the instance holding it, %s first or second.
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
		out = append(out, layerKeyLevel{"level 1 " + typ.name, "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n  v: Int = 0\n  u: Int = 0\n}\n\n" +
			"record A {\n  id: String\n  oo: " + typ.list + "\n}\n\n" +
			"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"{f}\" }] },\n" +
			"  { id: \"second\", oo: [{ k: \"{k}\" }] },\n]\n", "p:aa[second].oo[x]", "%s.oo"},
			layerKeyLevel{"level 2 " + typ.name, "package p\n\nrecord T {\n  k: String\n  w: Int = 0\n  v: Int = 0\n  u: Int = 0\n}\n\n" +
				"record S {\n  k: String\n  inn: " + typ.inn + "\n}\n\n" +
				"record A {\n  id: String\n  oo: " + typ.list + "\n}\n\n" +
				"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\", inn: [{ k: \"{f}\" }] }] },\n" +
				"  { id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"{k}\" }] }] },\n]\n", "p:aa[second].oo[x].inn[q]", "%s.oo.x.inn"})
	}
	return out
}

// old is the key l renames.
func (l layerKeyLevel) old() string {
	return l.path[strings.LastIndex(l.path, "[")+1 : len(l.path)-1]
}

// API.md R7, R11, E11, E22 (DECISIONS 316, 470, 483, 484): an amend path's `.x`, `["x"]` or bare
// `[x]` naming the renamed instance's entry, first's or second's, is one `layer` ref, active or
// not, rewritten once (`["a b"]` for a non-word); the other instance's is none, its rename applies.
func TestRefsLayerKeySegment(t *testing.T) {
	var rows []layerKeyRow
	for _, lines := range [][]string{{"first"}, {"second"}, {"first", "second"}, {"second", "second[]"}, {"first[]", "second[]"},
		{"first[k]"}, {"second[k]"}, {"first[k]", "second[k]"}, {"second", "second[]", "second[k]"}} {
		for _, of := range []string{"second", "first"} {
			for _, to := range []string{"y", "a b"} {
				rows = append(rows, layerKeyRow{true, lines, of, to}, layerKeyRow{false, lines, of, to})
			}
		}
	}
	for _, l := range layerKeyLevels() {
		for _, r := range rows {
			name := fmt.Sprintf("%s active %v %s rename %s to %q", l.name, r.active, strings.Join(r.lines, "+"), r.of, r.to)
			t.Run(name, func(t *testing.T) { layerKeyCase(t, l, r) })
		}
	}
}

// layerKeyRow is one row of TestRefsLayerKeySegment: the layer active or not, its lines, and the
// instance, first or second, whose entry is renamed to to.
type layerKeyRow struct {
	active bool
	lines  []string
	of, to string
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

// keyForm is how an amend segment writes a key.
type keyForm int

const (
	formDot    keyForm = iota // `.k`
	formString                // `["k"]`
	formBare                  // `[k]`
)

// layerKeyForms are the forms a layerKeyCase line writes its key in, by the suffix of its name,
// and the field each sets, so that no two lines of one instance overlap (E1908).
var layerKeyForms = []struct {
	suffix, field string
	form          keyForm
}{{"[]", ".v: 2", formString}, {"[k]", ".u: 3", formBare}, {"", ".w: 1", formDot}}

// layerKeySeg is key as an amend segment in form; `["k"]` for a key that is not a word (API.md E11).
func layerKeySeg(key string, form keyForm) string {
	switch {
	case form == formString || strings.Contains(key, " "):
		return "[\"" + key + "\"]"
	case form == formBare:
		return "[" + key + "]"
	}
	return "." + key
}

// layerKeyLine is one amend line of a layerKeyCase: the path to the keyed list, the instance it is
// through, the form it writes its key in and the field it sets.
type layerKeyLine struct {
	pre, inst string
	form      keyForm
	field     string
}

// layerKeyCase is one row of TestRefsLayerKeySegment: each of r.lines amends a field of the entry
// keyed by the old key in first's or second's instance, in the form its name's suffix gives
// (layerKeyForms); the rename is of r.of's entry to r.to.
func layerKeyCase(t *testing.T, l layerKeyLevel, r layerKeyRow) {
	var parts []layerKeyLine
	var want []string // R8: the layer file sorts before p's
	for i, who := range r.lines {
		for _, f := range layerKeyForms {
			if inst, ok := strings.CutSuffix(who, f.suffix); ok {
				parts = append(parts, layerKeyLine{fmt.Sprintf(l.pre, inst), inst, f.form, f.field})
				break
			}
		}
		if parts[i].inst == r.of {
			want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, strings.Count(layerKeyHead, "\n")+i+1))
		}
	}
	marks := map[string]string{"second": "{k}", "first": "{f}"}
	files := func(key string, quoted bool) map[string]string { // quoted: r.of's lines write `["k"]`
		layer := layerKeyHead
		for _, p := range parts {
			k, form := l.old(), p.form
			if p.inst == r.of {
				k = key
				if quoted {
					form = formString
				}
			}
			layer += "  " + p.pre + layerKeySeg(k, form) + p.field + "\n"
		}
		src := strings.ReplaceAll(l.src, marks[r.of], key)
		src = strings.NewReplacer(marks["first"], l.old(), marks["second"], l.old()).Replace(src)
		return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer+"}\n")}
	}
	var layers []string
	if r.active {
		layers = []string{"dev"}
	}
	path := strings.Replace(l.path, "second", r.of, 1)
	sr := sharedRow{l: sharedLevel{path: path}, layers: layers, before: files(l.old(), false), after: files(r.to, false), want: strings.Join(want, ", "), to: r.to}
	if strings.Contains(r.to, " ") {
		sr.undone = files(l.old(), true) // the renamed `["a b"]` is renamed back in its form (E11), equal as a value (E22)
	}
	sr.run(t)
}

// layerCopySrc is aa, whose second's oo holds an entry keyed k, and bb, a spread copy of second
// that keeps its entries (DECISIONS 410); a line on bb names second's entry through it.
func layerCopySrc(k string) string {
	return "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\n" +
		"record A {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }] },\n  { id: \"second\", oo: [{ k: \"" + k + "\" }] },\n]\n\n" +
		"let bb: A = { ...aa.second, id: \"third\" }\n"
}

// API.md R7, E11, E22 (DECISIONS 410, 473, 484): inactive, a key segment through a spread copy names
// the source's entry, a `layer` ref rewritten once, none of first's; active, the line it amends
// makes bb's x bb's own, whose ref it is once, and whose key the spread computes.
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
	sharedRow{l: sharedLevel{path: "p:bb.oo[x]"}, layers: []string{"dev"}, before: files("x"), want: want, refused: canon.ReasonComputed}.run(t)
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
