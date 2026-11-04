package edit_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/syntax"
)

// refKeyA holds lists keyed by a ref one level (us, vs, ds) and two (ts, ws, dds) down chains
// ending at a String (ss), a table id (es) and an enum (cs); a String-keyed list below a field
// (box); a let reading each element by its key.
const refKeyA = `/// A.
package a

/// An S.
record S {
  /// Id.
  id: String
  /// W.
  w: Int = 0
}

/// Ss.
let ss: [S] keyed by id = [{ id: "x" }, { id: "y" }]

/// A U.
record U {
  /// S.
  s: ref ss
  /// W.
  w: Int = 0
}

/// Us.
let us: [U] keyed by s = [{ s: x }]

/// A T.
record T {
  /// U.
  u: ref us
  /// W.
  w: Int = 0
}

/// Ts.
let ts: [T] keyed by u = [{ u: x }]

/// An E.
record E {
  /// W.
  w: Int = 0
}

/// Es.
let es: table E = {
  x {}
}

/// A V.
record V {
  /// E.
  e: ref es
  /// W.
  w: Int = 0
}

/// Vs.
let vs: [V] keyed by e = [{ e: x }]

/// A W2.
record W2 {
  /// V.
  v: ref vs
  /// W.
  w: Int = 0
}

/// Ws.
let ws: [W2] keyed by v = [{ v: x }]

/// A colour.
enum Color { x, y }

/// A C.
record C {
  /// Colour.
  c: Color
  /// W.
  w: Int = 0
}

/// Cs.
let cs: [C] keyed by c = [{ c: x }]

/// A D.
record D {
  /// C.
  c: ref cs
  /// W.
  w: Int = 0
}

/// Ds.
let ds: [D] keyed by c = [{ c: x }]

/// A DD.
record DD {
  /// D.
  d: ref ds
  /// W.
  w: Int = 0
}

/// Dds.
let dds: [DD] keyed by d = [{ d: x }]

/// A box.
record Box {
  /// Its items.
  items: [S] keyed by id = []
}

/// A box.
let box: Box = { items: [{ id: "x" }] }

/// Reads every element by its key.
let r: Int = ss.x.w + us.x.w + ts.x.w + es.x.w + vs.x.w + ws.x.w
`

// refKeyDev amends a line below an element of each String- or table-leaf list, a String-keyed
// element whole, and a key below a field.
const refKeyDev = `package a
layer dev

amend ss {
  x.w: 3
  y: { id: "y", w: 7 }
}

amend us {
  x.w: 1
}

amend ts {
  x.w: 2
}

amend es {
  x.w: 3
}

amend vs {
  x.w: 4
}

amend ws {
  x.w: 5
}

amend box {
  items["x"].w: 6
}
`

const refKeyLayerFile = "law/a/dev.layer.canon"

// refKeyLaw is refKeyA with a dev layer that amends only ss.
var refKeyLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon":     file(refKeyA),
	refKeyLayerFile:     file("package a\nlayer dev\n\namend ss {\n  y.w: 1\n}\n"),
}

// refKeyRename is refKeyA with refKeyDev.
var refKeyRename = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon":     file(refKeyA),
	refKeyLayerFile:     file(refKeyDev),
}

// API.md 6.4, W11 (EVALUATION.md 9.2; DECISIONS 450, 456): a Set at a word key of a list keyed by a
// ref chain ending at a String or a table id, one or two levels down, writes a new line that key
// starts, bare; layer active or not, the result checks clean both ways and the Undo restores it.
func TestDecisions456RefKeyStartsLine(t *testing.T) {
	cases := []struct{ name, root string }{
		{"String leaf, one level", "us"},
		{"String leaf, two levels", "ts"},
		{"table leaf, one level", "vs"},
		{"table leaf, two levels", "ws"},
	}
	for _, c := range cases {
		for _, layers := range [][]string{{"dev"}, nil} {
			t.Run(c.name+" layers="+strings.Join(layers, ","), func(t *testing.T) {
				op := edit.Operation{Kind: edit.OpSet, Path: "a:" + c.root + "[x].w", Value: edit.Int(5)}
				s := open(t, refKeyLaw, layers, "dev", "a")
				plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
				if err != nil {
					t.Fatalf("Set %s: %v, want it applied (DECISIONS 456)", op.Path, err)
				}
				for _, ch := range plan.Changes {
					checkWritten(t, plan, ch)
				}
				after := written(refKeyLaw, plan)
				want := "package a\nlayer dev\n\namend ss {\n  y.w: 1\n}\n\namend " + c.root + " { x.w: 5 }\n"
				if got := fileText(after, refKeyLayerFile); got != want {
					t.Errorf("%s holds:\n%s\nwant:\n%s", refKeyLayerFile, got, want)
				}
				if len(plan.Changes) != 1 {
					t.Errorf("changes %v, want only %s written", plan.Changes, refKeyLayerFile)
				}
				checkRefKeyClean(t, after)
				if got := values(open(t, after, []string{"dev"}, "", "a"), false)["a:"+c.root]; !strings.Contains(got, "w: 5") {
					t.Errorf("with dev, %s = %s, want its x element at w: 5", c.root, got)
				}
				g := goldenCase{fsys: refKeyLaw, pkgs: []string{"a"}, layers: layers, editLayer: "dev"}
				checkUndo(t, g, []edit.Operation{op}, plan)
				checkUndoFile(t, g, plan, refKeyLayerFile)
			})
		}
	}
}

// API.md 6.4 (DECISIONS 450, 456): Editable and Apply agree: a word key of a list keyed by a ref
// whose chain ends at a String or a table id starts a line; one whose chain ends at an enum,
// one level down or two, stays `layer`.
func TestDecisions456RefKeyLineRefusals(t *testing.T) {
	cases := []struct {
		path string
		want edit.Reason
	}{
		{"a:us[x].w", edit.ReasonNone},
		{"a:ts[x].w", edit.ReasonNone},
		{"a:vs[x].w", edit.ReasonNone},
		{"a:ws[x].w", edit.ReasonNone},
		{"a:ds[x].w", edit.ReasonLayer},
		{"a:dds[x].w", edit.ReasonLayer},
		{"a:cs[x].w", edit.ReasonLayer},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			s := open(t, refKeyLaw, []string{"dev"}, "dev", "a")
			f := fixture{Snapshot: s.snap, files: s.a.Files()}
			op := edit.Operation{Kind: edit.OpSet, Path: c.path, Value: edit.Int(5)}
			checkVerdict(t, s, resolve(t, f, c.path), op, c.want)
		})
	}
}

// API.md E11, E22, W10 (DECISIONS 310; EVALUATION.md 9.2): a Rename rewrites only the key segments and
// whole-element key fields naming its elements, cascaded ones included, in every layer, active or not;
// it checks clean both ways, its Undo restores the file; an element an active layer states is `layered`.
func TestDecisions310RenameRewritesLayerKeys(t *testing.T) {
	cases := []struct {
		name, path, key string
		replaced        []string // old, new pairs of refKeyDev's lines
		layered         bool     // refused while dev is active
	}{
		{"String key, cascading two levels", "a:ss[x]", "z", []string{
			"amend ss {\n  x.w: 3", "amend ss {\n  z.w: 3",
			"amend us {\n  x.w", "amend us {\n  z.w",
			"amend ts {\n  x.w", "amend ts {\n  z.w"}, false},
		{"table entry, cascading two levels", "a:es.x", "z", []string{
			"amend es {\n  x.w", "amend es {\n  z.w",
			"amend vs {\n  x.w", "amend vs {\n  z.w",
			"amend ws {\n  x.w", "amend ws {\n  z.w"}, false},
		{"an element stated whole", "a:ss[y]", "q", []string{`y: { id: "y", w: 7 }`, `q: { id: "q", w: 7 }`}, true},
		{"a key below a field", "a:box.items[x]", "z", []string{`items["x"]`, `items["z"]`}, false},
		{"a key below a field, no word", "a:box.items[x]", "z z", []string{`items["x"]`, `items["z z"]`}, false},
	}
	for _, c := range cases {
		for _, layers := range [][]string{nil, {"dev"}} {
			t.Run(c.name+" layers="+strings.Join(layers, ","), func(t *testing.T) {
				op := edit.Operation{Kind: edit.OpRename, Path: c.path, Key: edit.Key(c.key)}
				s := open(t, refKeyRename, layers, "", "a")
				plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
				if c.layered && len(layers) > 0 {
					checkReason(t, err, edit.ReasonLayered)
					return
				}
				if err != nil {
					t.Fatalf("Rename %s to %s: %v, want it applied (DECISIONS 310)", c.path, c.key, err)
				}
				for _, ch := range plan.Changes {
					checkWritten(t, plan, ch)
				}
				after := written(refKeyRename, plan)
				if got, want := fileText(after, refKeyLayerFile), strings.NewReplacer(c.replaced...).Replace(refKeyDev); got != want {
					t.Errorf("%s holds:\n%s\nwant:\n%s", refKeyLayerFile, got, want)
				}
				checkRefKeyClean(t, after)
				g := goldenCase{fsys: refKeyRename, pkgs: []string{"a"}, layers: layers}
				checkUndo(t, g, []edit.Operation{op}, plan)
				checkUndoFile(t, g, plan, refKeyLayerFile)
			})
		}
	}
}

// API.md 6.4, W11 (DECISIONS 305, 310; GRAMMAR.md 3.1 rule 3): a Rename leaving a line that starts
// with a key no amend line can spell (no word, or a word continuing the line above) is `layer`,
// naming the line, its file and layer; under an edit layer every Rename is `layer`.
func TestDecisions310RenameLayerRefusals(t *testing.T) {
	cases := []struct {
		key, editLayer string
		layers         []string
		line           bool // the refusal names dev's line
	}{
		{"z z", "", nil, true},
		{"z z", "", []string{"dev"}, true},
		{"in", "", nil, true},
		{"where", "", []string{"dev"}, true},
		{"z", "dev", nil, false},
		{"z", "dev", []string{"dev"}, false},
	}
	for _, c := range cases {
		t.Run(c.key+" "+c.editLayer+" layers="+strings.Join(c.layers, ","), func(t *testing.T) {
			s := open(t, ssAlone, c.layers, c.editLayer, "a")
			op := edit.Operation{Kind: edit.OpRename, Path: "a:ss[x]", Key: edit.Key(c.key)}
			_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
			ne := checkReason(t, err, edit.ReasonLayer)
			if c.line && (ne == nil || ne.Layer != "dev" || ne.File != "a/dev.layer.canon" || !slices.Equal(ne.Refs, []string{"a:ss.x.w"})) {
				t.Errorf("refusal %+v, want it to name dev's line a:ss.x.w in a/dev.layer.canon (DECISIONS 305)", ne)
			}
		})
	}
}

// ssAlone is ss alone, no code reading its keys, and a dev layer whose second line amends ss[x].
var ssAlone = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon":     file(refKeyA[:strings.Index(refKeyA, "/// A U.")]),
	refKeyLayerFile:     file("package a\nlayer dev\n\namend ss {\n  y.w: 1\n  x.w: 3\n}\n"),
}

// checkReason is err a refusal with reason want, returned.
func checkReason(t *testing.T, err error, want edit.Reason) *edit.NotEditableError {
	t.Helper()
	var ne *edit.NotEditableError
	if !errors.As(err, &ne) || ne.Reason != want {
		t.Errorf("got %v, want a refusal with reason %s", err, parityReasons[want])
		return nil
	}
	return ne
}

// API.md 6.4, W11 (DECISIONS 310, 450; GRAMMAR.md 2.3, 3.1 rule 3, 5.7): every reserved word starts
// an amend line, of a table or a String-keyed list, written by a Set or a Rename, except the six that
// continue the line above (`and or in is else where`), which are `layer`; what is written checks clean.
func TestReservedWordsStartLines(t *testing.T) {
	for k := syntax.KwAnd; k < syntax.TokenKindCount; k++ {
		w := k.String()
		cases := wordCases(w)
		if slices.Contains([]string{"true", "false", "none", "self"}, w) {
			cases = cases[:len(cases)/2] // no table key (E1126): its String-keyed list rows only
		}
		continues := slices.Contains([]string{"and", "or", "in", "is", "else", "where"}, w)
		for _, c := range cases {
			t.Run(w+" "+c.name, func(t *testing.T) {
				s := open(t, c.fsys, nil, c.editLayer, "a")
				plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
				if continues {
					checkReason(t, err, edit.ReasonLayer)
					return
				}
				if err != nil {
					t.Fatalf("%v: %v, want it written", c.op, err)
				}
				checkRefKeyClean(t, written(c.fsys, plan))
			})
		}
	}
}

// wordCase is an op that writes word w at the start of a second line of dev.
type wordCase struct {
	name, editLayer string
	fsys            mapFS
	op              edit.Operation
}

// wordCases are a Set and a Rename that start a line with w, on a String-keyed list and a table.
func wordCases(w string) []wordCase {
	list := "package a\n\n/// An S.\nrecord S {\n  /// Id.\n  id: String\n  /// W.\n  w: Int = 0\n}\n\n/// Ss.\nlet ss: [S] keyed by id = [{ id: \"k\" }, { id: \"x\" }, { id: \"" + w + "\" }]\n"
	table := "package a\n\n/// An E.\nrecord E {\n  /// W.\n  w: Int = 0\n}\n\n/// Tb.\nlet tb: table E = {\n  k {}\n}\n\n/// X.\nentry tb.x {}\n\n/// Its last entry.\nentry tb." + w + " {}\n"
	law := func(src, root string, lines ...string) mapFS {
		return mapFS{
			"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
			"law/a/a.canon":     file("/// A.\n" + src),
			refKeyLayerFile:     file("package a\nlayer dev\n\namend " + root + " {\n  " + strings.Join(lines, "\n  ") + "\n}\n"),
		}
	}
	renamedList := strings.Replace(list, `, { id: "`+w+`" }`, "", 1)
	renamedTable := strings.Replace(table, "\n\n/// Its last entry.\nentry tb."+w+" {}", "", 1)
	return []wordCase{
		{"list Set", "dev", law(list, "ss", "k.w: 1"), edit.Operation{Kind: edit.OpSet, Path: "a:ss[" + w + "].w", Value: edit.Int(5)}},
		{"list Rename", "", law(renamedList, "ss", "k.w: 1", "x.w: 3"), edit.Operation{Kind: edit.OpRename, Path: "a:ss[x]", Key: edit.Key(w)}},
		{"table Set", "dev", law(table, "tb", "k.w: 1"), edit.Operation{Kind: edit.OpSet, Path: "a:tb." + w + ".w", Value: edit.Int(5)}},
		{"table Rename", "", law(renamedTable, "tb", "k.w: 1", "x.w: 3"), edit.Operation{Kind: edit.OpRename, Path: "a:tb.x", Key: edit.Key(w)}},
	}
}

// refKeyMore is refKeyA with maps of Ints and of records below a field, and a list keyed by an
// Int holding a negative key.
const refKeyMore = refKeyA + `
/// An R.
record R {
  /// W.
  w: Int = 0
}

/// A holder.
record H {
  /// Ints.
  m: {String: Int} = {}
  /// Records.
  rm: {String: R} = {}
}

/// A holder.
let holder: H = { m: { "a": 1 }, rm: { "a": { w: 0 } } }

/// An NI.
record NI {
  /// N.
  n: Int
  /// W.
  w: Int = 0
}

/// An NB.
record NB {
  /// Items.
  items: [NI] keyed by n = []
}

/// An NB.
let nb: NB = { items: [{ n: -1 }, { n: 5 }] }
`

// API.md E11, E22, 6.4, W10 (DECISIONS 310; EVALUATION.md 9.2): a Rename rewrites only its keys: a keyed
// list's negative key is no position; a later `.k` no word becomes `["…"]` (its Undo keeps that form);
// `[#n]`, comments, other bytes kept; map keys follow; an entry an active layer states is `layered`.
func TestDecisions310RenameLayerKeyForms(t *testing.T) {
	cases := []struct {
		name, lines, path  string
		key                edit.Lit
		want               string
		activeToo, spelled bool // also renamed with dev active; its Undo restores the value, not the bytes
	}{
		{"a negative key, another renamed", "nb {\n  items[-1].w: 3\n}", "a:nb.items[5]", edit.Int(6), "nb {\n  items[-1].w: 3\n}", true, false},
		{"a negative key renamed", "nb {\n  items[-1].w: 3\n}", "a:nb.items[-1]", edit.Int(7), "nb {\n  items[7].w: 3\n}", true, false},
		{"a later .k, no word", "box {\n  items.x.w: 6\n}", "a:box.items[x]", edit.Key("z z"), "box {\n  items[\"z z\"].w: 6\n}", true, true},
		{"a later .k, a keyword", "box {\n  items.x.w: 6\n}", "a:box.items[x]", edit.Key("in"), "box {\n  items.in.w: 6\n}", true, false},
		{"[#n] kept", "box {\n  items[#0].w: 6\n}", "a:box.items[x]", edit.Key("z"), "box {\n  items[#0].w: 6\n}", true, false},
		{"a map key", "holder {\n  m[\"a\"]: 3\n}", "a:holder.m[a]", edit.Key("c"), "holder {\n  m[\"c\"]: 3\n}", false, false},
		{"a key below a map entry", "holder {\n  rm[\"a\"].w: 3\n}", "a:holder.rm[a]", edit.Key("c"), "holder {\n  rm[\"c\"].w: 3\n}", true, false},
		{"comments kept", "ss {\n  // x's weight.\n  x.w: 3 // three\n}", "a:ss[x]", edit.Key("z"), "ss {\n  // x's weight.\n  z.w: 3 // three\n}", true, false},
		{"a ref-keyed element stated whole", "us {\n  x: { s: x, w: 9 }\n}", "a:ss[x]", edit.Key("z"), "us {\n  z: { s: z, w: 9 }\n}", false, false},
	}
	for _, c := range cases {
		sets := [][]string{nil}
		if c.activeToo {
			sets = append(sets, []string{"dev"})
		}
		for _, layers := range sets {
			t.Run(c.name+" layers="+strings.Join(layers, ","), func(t *testing.T) {
				fsys := mapFS{
					"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
					"law/a/a.canon":     file(refKeyMore),
					refKeyLayerFile:     file("package a\nlayer dev\n\n// dev's lines.\namend " + c.lines + "\n"),
				}
				op := edit.Operation{Kind: edit.OpRename, Path: c.path, Key: c.key}
				s := open(t, fsys, layers, "", "a")
				plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
				if err != nil {
					t.Fatalf("Rename %s to %v: %v, want it applied (DECISIONS 310)", c.path, c.key, err)
				}
				after := written(fsys, plan)
				if got, want := fileText(after, refKeyLayerFile), "package a\nlayer dev\n\n// dev's lines.\namend "+c.want+"\n"; got != want {
					t.Errorf("%s holds:\n%s\nwant:\n%s", refKeyLayerFile, got, want)
				}
				checkRefKeyClean(t, after)
				g := goldenCase{fsys: fsys, pkgs: []string{"a"}, layers: layers}
				checkUndo(t, g, []edit.Operation{op}, plan)
				want := fileText(fsys, refKeyLayerFile)
				if c.spelled { // the Undo writes the old key in the form the new one took
					want = strings.Replace(want, "items.x", `items["x"]`, 1)
				}
				if got := undoneText(t, after, layers, plan); got != want {
					t.Errorf("Undo: %s holds:\n%s\nwant:\n%s", refKeyLayerFile, got, want)
				}
			})
		}
	}
}

// API.md W11, W11a, E22 (EVALUATION.md 9.2): a keyed list's negative key in an amend path is a key,
// never a position: a Set at it edits the line naming it in place, or writes a new line naming it;
// layer active or not, the result checks clean both ways and the Undo restores the file.
func TestW11aNegativeKeyLine(t *testing.T) {
	cases := []struct{ name, before, after string }{
		{"the line naming it", "amend nb {\n  items[-1].w: 9\n}\n", "amend nb {\n  items[-1].w: 5\n}\n"},
		{"a new line", "amend ss {\n  y.w: 1\n}\n", "amend ss {\n  y.w: 1\n}\n\namend nb { items[-1].w: 5 }\n"},
	}
	for _, c := range cases {
		for _, layers := range [][]string{nil, {"dev"}} {
			t.Run(c.name+" layers="+strings.Join(layers, ","), func(t *testing.T) {
				fsys := mapFS{
					"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
					"law/a/a.canon":     file(refKeyMore),
					refKeyLayerFile:     file("package a\nlayer dev\n\n" + c.before),
				}
				op := edit.Operation{Kind: edit.OpSet, Path: "a:nb.items[-1].w", Value: edit.Int(5)}
				s := open(t, fsys, layers, "dev", "a")
				plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
				if err != nil {
					t.Fatalf("Set %s: %v, want it written (API.md W11a)", op.Path, err)
				}
				after := written(fsys, plan)
				if got, want := fileText(after, refKeyLayerFile), "package a\nlayer dev\n\n"+c.after; got != want {
					t.Errorf("%s holds:\n%s\nwant:\n%s", refKeyLayerFile, got, want)
				}
				checkRefKeyClean(t, after)
				g := goldenCase{fsys: fsys, pkgs: []string{"a"}, layers: layers, editLayer: "dev"}
				checkUndo(t, g, []edit.Operation{op}, plan)
				checkUndoFile(t, g, plan, refKeyLayerFile)
			})
		}
	}
}

// undoneText is dev's layer file once the plan's Undo is applied to after, the state it left (E22).
func undoneText(t *testing.T, after mapFS, layers []string, plan *edit.Plan) string {
	t.Helper()
	s := open(t, after, layers, "", "a")
	undo, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Fatalf("Undo %v: %v", plan.Undo, err)
	}
	return fileText(written(after, undo), refKeyLayerFile)
}

// checkRefKeyClean opens fsys without a layer and with dev: no error finding either way (API.md E18).
func checkRefKeyClean(t *testing.T, fsys mapFS) {
	t.Helper()
	for _, layers := range [][]string{nil, {"dev"}} {
		s := open(t, fsys, layers, "", "a")
		for _, f := range s.a.Bag("a").Findings() {
			if f.Severity == diag.Error {
				t.Errorf("after the edit, layers %v: %s %s %s", layers, f.Code, f.Path, f.Message)
			}
		}
	}
}
