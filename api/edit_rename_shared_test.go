package canon_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// sharedLayer is the layer file the layer-line cases amend aa in.
const sharedLayer = "p/dev.layer.canon"

// sharedLevel is aa with pick a ref into each instance's own list at one level, first holding x
// and y, second x: {k} marks second's renamed key and its tokens, {pf} {ps} {def} the picks and
// default; the lines amend each pick, and first and second (of rec, without pick) at their paths whole.
type sharedLevel struct {
	name, src, path, old, firstLine, secondLine string
	firstPath, rec, first, secondPath, second   string
}

// sharedLevels are a level-1 and a level-2 ref into the instance's own keyed list.
var sharedLevels = []sharedLevel{
	{"level 1", "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\n" +
		"record A {\n  id: String\n  oo: [S] keyed by k\n  pick: ref S{def}\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }, { k: \"y\" }]{pf} },\n" +
		"  { id: \"second\", oo: [{ k: \"{k}\" }]{ps} },\n]\n", "p:aa[second].oo[x]", "x", "first.pick", "second.pick",
		"first", "A", "{ id: \"first\", oo: [{ k: \"x\" }, { k: \"y\" }] }", "second", "{ id: \"second\", oo: [{ k: \"{k}\" }] }"},
	{"level 2", "package p\n\nrecord T {\n  k: String\n  w: Int = 0\n}\n\n" +
		"record S {\n  k: String\n  inn: [T] keyed by k\n  pick: ref T{def}\n}\n\n" +
		"record A {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\", inn: [{ k: \"q\" }, { k: \"y\" }]{pf} }] },\n" +
		"  { id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"{k}\" }]{ps} }] },\n]\n", "p:aa[second].oo[x].inn[q]", "q", "first.oo.x.pick", "second.oo.x.pick",
		"first.oo.x", "S", "{ k: \"x\", inn: [{ k: \"q\" }, { k: \"y\" }] }", "second.oo.x", "{ k: \"x\", inn: [{ k: \"{k}\" }] }"},
}

// picked is first's instance with its pick naming its own entry of the old key.
func (l sharedLevel) picked() string {
	return withPick(l.first, l.old)
}

// withPick is the instance literal inst with a pick of key.
func withPick(inst, key string) string {
	return strings.TrimSuffix(inst, " }") + ", pick: \"" + key + "\" }"
}

// fill is l's source with each mark replaced, {k} by key.
func (l sharedLevel) fill(text, key string, marks map[string]string) string {
	for _, m := range slices.Sorted(maps.Keys(marks)) {
		text = strings.ReplaceAll(text, m, marks[m])
	}
	return strings.ReplaceAll(text, "{k}", key)
}

// sharedOpen opens editLaw with files, layers active.
func sharedOpen(t *testing.T, files map[string]string, layers []string) (*canon.Project, *memFS) {
	t.Helper()
	law := maps.Clone(editLaw)
	maps.Copy(law, files)
	opts := project(law)
	opts.Layers = layers
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	return p, m
}

// sharedRefs is each ref's kind, file and line, in R8 order.
func sharedRefs(refs []canon.Ref) string {
	var out []string
	for _, r := range refs {
		out = append(out, fmt.Sprintf("%s %s:%d", r.Kind, r.File, r.Line))
	}
	return strings.Join(out, ", ")
}

// sharedRow renames l's entry to y, or to, with layers active: want is its Refs, after the files
// the rename leaves of before, refused the reason it fails with, writing nothing ("" for none),
// undone the files its Undo leaves when not before's bytes (E22 restores values, not text).
type sharedRow struct {
	l                     sharedLevel
	layers                []string
	before, after, undone map[string]string
	want                  string
	refused               canon.Reason
	to                    string
	key                   canon.Lit // the new key when not a string key (a member)
}

// run checks r's Refs and rename, then that the Undo gives every file back byte for byte.
func (r sharedRow) run(t *testing.T) {
	t.Helper()
	p, m := sharedOpen(t, r.before, r.layers)
	ctx := context.Background()
	refs, err := p.Refs(ctx, r.l.path)
	if err != nil {
		t.Fatalf("API.md R7: Refs: %v", err)
	}
	if got := sharedRefs(refs.Refs); got != r.want {
		t.Errorf("API.md R7 (DECISIONS 483): the refs of %s are\n%q, want\n%q", r.l.path, got, r.want)
	}
	key := r.key
	if key == nil {
		key = canon.Key(cmp.Or(r.to, "y"))
	}
	res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(r.l.path, key)}})
	var ne *canon.NotEditableError
	after := r.after
	switch {
	case r.refused != "" && (!errors.As(err, &ne) || ne.Reason != r.refused):
		t.Fatalf("API.md E12: the rename gives %v, want a NotEditableError reason %s", err, r.refused)
	case r.refused != "":
		after = r.before // nothing is written (E19)
	case err != nil:
		t.Fatalf("API.md E19: the rename is refused: %v", err)
	}
	before := r.before
	for _, name := range slices.Sorted(maps.Keys(before)) {
		if got := read(t, m, name); got != after[name] {
			t.Errorf("API.md E11, R7: %s after the rename:\n%s\nwant:\n%s", name, got, after[name])
		}
	}
	if r.refused != "" {
		return
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E22: the Undo: %v", err)
	}
	undone := before
	if r.undone != nil {
		undone = r.undone
	}
	for _, name := range slices.Sorted(maps.Keys(before)) {
		if got := read(t, m, name); got != undone[name] {
			t.Errorf("API.md E22: %s after the Undo:\n%s", name, got)
		}
	}
}

// API.md R7, E11, E22 (DECISIONS 316, 473, 483): a ref stated in an amendment's right-hand side
// names the entry of its bound instance, the layer active or not: a line on first's pick, or one
// building first whole, is left; a line on second's pick names the renamed entry and is rewritten.
func TestEditRenameLayerLineStatedRef(t *testing.T) {
	for _, l := range sharedLevels {
		for _, active := range []bool{true, false} {
			for _, lines := range [][]string{{"first"}, {"second"}, {"first", "second"}, {"built"}, {"built", "second"}, {"rebuilt"}} {
				name := fmt.Sprintf("%s active %v %s", l.name, active, strings.Join(lines, "+"))
				t.Run(name, func(t *testing.T) { layerLineCase(t, l, active, lines) })
			}
		}
	}
}

// layerLineCase is one row of TestEditRenameLayerLineStatedRef: first's pick states y, second's
// the renamed key, and the layer sets lines' picks to the old key.
func layerLineCase(t *testing.T, l sharedLevel, active bool, lines []string) {
	marks := map[string]string{"{def}": "", "{pf}": ", pick: \"y\"", "{ps}": ", pick: \"{k}\""}
	layer := "package p\nlayer dev\n\namend aa {\n"
	var want []string // R8: the layer file sorts before p's
	for _, who := range lines {
		line := l.firstLine + ": \"" + l.old + "\""
		switch who {
		case "second":
			line = l.secondLine + ": \"{k}\""
			want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, strings.Count(layer, "\n")+1))
		case "built":
			line = l.firstPath + ": " + l.picked()
		case "rebuilt": // second's own instance whole: its key and its pick both name the renamed entry
			line = l.secondPath + ": " + withPick(l.second, "{k}")
			want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, strings.Count(layer, "\n")+1))
		}
		layer += "  " + line + "\n"
	}
	layer += "}\n"
	row := sharedRow{l: l}
	if rebuilt := slices.Contains(lines, "rebuilt"); rebuilt && active {
		row.refused = canon.ReasonLayered // the renamed entry is the layer's own (W11)
	} else {
		want = append(want, fmt.Sprintf("value %s:%d", srcMain, lineOf(l.fill(l.src, l.old, marks), "  { id: \"second\"")))
	}
	files := func(key string) map[string]string {
		return map[string]string{srcMain: l.fill(l.src, key, marks), sharedLayer: l.fill(layer, key, nil)}
	}
	var layers []string
	if active {
		layers = []string{"dev"}
	}
	row.layers, row.before, row.after, row.want = layers, files(l.old), files("y"), strings.Join(want, ", ")
	row.run(t)
}

// API.md R7, E11, E12, E19, E22 (DECISIONS 473, 483): a default typed `ref C` into each instance's
// own list is a ref of second's entry only where it names it: rewritten when only second takes it,
// unlisted when second does not, and the rename refused `computed` when first takes it too.
func TestEditRenameSharedDefaultRef(t *testing.T) {
	cases := []struct {
		name          string
		first, second bool // each instance takes the default
		code          bool
		refused       canon.Reason
	}{
		{name: "only first takes it", first: true},
		{name: "only second takes it", second: true, code: true},
		{name: "both take it", first: true, second: true, code: true, refused: canon.ReasonComputed},
		{name: "neither takes it"},
	}
	for _, l := range sharedLevels {
		for _, c := range cases {
			t.Run(l.name+" "+c.name, func(t *testing.T) {
				def := "{k}"
				if !c.code {
					def = l.old
				}
				marks := map[string]string{"{def}": " = \"" + def + "\"", "{pf}": ", pick: \"y\"", "{ps}": ", pick: \"{k}\""}
				if c.first {
					marks["{pf}"] = ""
				}
				if c.second {
					marks["{ps}"] = ""
				}
				src := l.fill(l.src, l.old, marks)
				var want []string
				if c.code {
					want = append(want, fmt.Sprintf("code %s:%d", srcMain, lineOf(src, "  pick: ref")))
				}
				if !c.second {
					want = append(want, fmt.Sprintf("value %s:%d", srcMain, lineOf(src, "  { id: \"second\"")))
				}
				before, after := map[string]string{srcMain: src}, map[string]string{srcMain: l.fill(l.src, "y", marks)}
				sharedRow{l: l, before: before, after: after, want: strings.Join(want, ", "), refused: c.refused}.run(t)
			})
		}
	}
}

// API.md R7, E11, E22 (DECISIONS 483): a constant's token typed `ref C` into its instance's own
// keyed list names the entry of each instance it builds: first built from the constant names
// first's entry, so the constant is no ref of second's and is left; an unused one is none either.
func TestEditRenameSharedConstRef(t *testing.T) {
	marks := map[string]string{"{def}": "", "{pf}": "", "{ps}": ", pick: \"{k}\""}
	for _, l := range sharedLevels {
		for _, used := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s used %v", l.name, used), func(t *testing.T) {
				konst := "const CF = " + l.rec + " " + l.picked() + "\n\n"
				with := func(key string) map[string]string {
					src, first := l.fill(l.src, key, marks), l.picked()
					if used {
						first = "CF"
					}
					src = strings.Replace(src, l.first, first, 1)
					out, err := canon.Format(srcMain, []byte(strings.Replace(src, "let aa", konst+"let aa", 1)))
					if err != nil {
						t.Fatalf("the fixture does not format: %v", err)
					}
					return map[string]string{srcMain: string(out)}
				}
				before := with(l.old)
				second := slices.IndexFunc(strings.Split(before[srcMain], "\n"), func(s string) bool { return strings.Contains(s, "id: \"second\"") })
				want := fmt.Sprintf("value %s:%d", srcMain, second+1)
				sharedRow{l: l, before: before, after: with("y"), want: want}.run(t)
			})
		}
	}
}
