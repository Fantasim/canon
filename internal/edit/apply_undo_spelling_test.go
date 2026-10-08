package edit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/edit"
)

// spellSrc spells its scalars other than in their canonical text (DECISIONS 337).
const spellSrc = `package d

/// A tone.
enum Tone { calm, danger }

/// Where a skill lands.
record Pos {
  /// Across.
  x: Float
  /// Down.
  y: Float
}

/// A skill.
record Skill {
  /// Its power multiplier.
  mul: Float
  /// Its scale.
  big: Float
  /// Its rate.
  rate: Float
  /// Its mask.
  mask: Int
  /// Its cost.
  cost: Int
  /// Its cooldown.
  cd: Duration
  /// Its label.
  label: String
  /// Its tone.
  tone: Tone
  /// Where it lands.
  at: Pos
  /// Its steps.
  steps: [Float]
  /// A note.
  note: String = ""
}

/// The skills.
let skills: table Skill = {
  slash {
    mul: 1.0
    big: 1e3
    rate: 0.50
    mask: 0x1F
    cost: 1_000
    cd: 1m30s
    label: "a\tb\u{41}\"c"
    tone: danger
    at: { x: 2.50, y: -1.0 }
    steps: [1.0, 2.50, 3]
  }
  bash {
    mul: 2.0
    big: 1E2
    rate: 0.25
    mask: 0b101
    cost: 10_0
    cd: 2s
    label: r"x\y"
    tone: calm
    at: { x: 0.0, y: 1e1 }
    steps: []
  }
}

/// A mark whose place has a default.
record Mark {
  /// Where.
  at: Pos = { x: 2.50, y: 1.0 }
  /// A tag.
  tag: String = ""
}

/// A base value.
let base: Float = 1.50

/// A spot naming a value.
let named: Pos = { x: base, y: 2.0 }

/// An origin.
let origin: Pos = { x: 2.50, y: 1.0 }

/// A spot spreading the origin.
let spread: Pos = { ...origin, y: 2.0 }

/// A mark left at its default place.
let mark: Mark = { tag: "m" }

/// A spot computed from literals.
let picked: Pos = { x: [2.50, 1.0][0], y: if true { 3.0 } else { 4.0 } }

/// A spot whose place a match picks.
let toned: Pos = { x: match Tone.danger { calm => 1.50, danger => 2.50 }, y: 2.0 }

/// A spot written with a comment inside and a computed field.
let spot: Pos = {
  // across
  x: 0.5 * 2
  y: 2.0
}
`

func spellFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(spellSrc)}
}

// DECISIONS 337, API.md §8.7 E22: an edit's Undo, and its Undo, give each file back byte for byte.
func TestUndoKeepsLiteralSpelling(t *testing.T) {
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"Float 1.0", []edit.Operation{setAt("skills.slash.mul", edit.Int(2))}},
		{"Float 1e3", []edit.Operation{setAt("skills.slash.big", edit.Float(5))}},
		{"Float 1E2", []edit.Operation{setAt("skills.bash.big", edit.Float(5))}},
		{"Float 0.50", []edit.Operation{setAt("skills.slash.rate", edit.Float(0.75))}},
		{"Int 0x1F", []edit.Operation{setAt("skills.slash.mask", edit.Int(3))}},
		{"Int 0b101", []edit.Operation{setAt("skills.bash.mask", edit.Int(3))}},
		{"Int 1_000", []edit.Operation{setAt("skills.slash.cost", edit.Int(7))}},
		{"Duration", []edit.Operation{setAt("skills.slash.cd", edit.Dur(5*time.Second))}},
		{"a Duration's Source, in canonical text (API.md M7)", []edit.Operation{setAt("skills.bash.cd", edit.Source("90s"))}},
		{"string with escapes", []edit.Operation{setAt("skills.slash.label", edit.Str("z"))}},
		{"raw string", []edit.Operation{setAt("skills.bash.label", edit.Str("z"))}},
		{"enum member", []edit.Operation{setAt("skills.slash.tone", edit.Member("calm"))}},
		{"nested record field", []edit.Operation{setAt("skills.slash.at.x", edit.Float(9))}},
		{"negative", []edit.Operation{setAt("skills.slash.at.y", edit.Float(4))}},
		{"list element", []edit.Operation{setAt("skills.slash.steps[1]", edit.Float(9))}},
		{"list element removed", []edit.Operation{{Kind: edit.OpRemove, Path: "skills.slash.steps[0]"}}},
		{"several ops", []edit.Operation{setAt("skills.slash.mul", edit.Int(2)), setAt("skills.slash.mask", edit.Source("0x20")),
			setAt("skills.bash.at.y", edit.Float(3))}},
		{"an op's own spelling", []edit.Operation{setAt("skills.slash.mul", edit.Source("3.0")), setAt("skills.slash.cost", edit.Source("2_000"))}},
	} {
		undoTwiceFiles(t, c.name, spellFS(), c.ops)
	}
}

// DECISIONS 337, API.md E22, E23: the Undo's Source spells each literal token as written.
func TestUndoCarriesLiteralSpelling(t *testing.T) {
	for _, c := range []struct {
		name, path string
		v          edit.Lit
		want       edit.Source
	}{
		{"Float", "skills.slash.mul", edit.Int(2), "1.0"},
		{"hex", "skills.slash.mask", edit.Int(3), "0x1F"},
		{"string", "skills.slash.label", edit.Str("z"), `"a\tb\u{41}\"c"`},
		{"nested", "skills.slash.at.y", edit.Float(4), "-1.0"},
		{"canonical already", "skills.bash.cd", edit.Dur(time.Second), "2s"},
		{"composite: its literals as written", "skills.slash.at", edit.Source("{ x: 1, y: 1 }"), "{ x: 2.50, y: -1.0 }"},
		{"composite: computed canonical, comments not restored", "spot", edit.Source("{ x: 3, y: 3 }"), "{ x: 1, y: 2.0 }"},
		{"a value a name brings: canonical", "named", edit.Source("{ x: 3, y: 3 }"), "{ x: 1.5, y: 2.0 }"},
		{"a value a spread brings: canonical", "spread.x", edit.Float(9), "2.5"},
		{"a whole value with a spread: canonical there", "spread", edit.Source("{ x: 3, y: 3 }"), "{ x: 2.5, y: 2.0 }"},
		{"a value a default brings: canonical", "mark.at.x", edit.Float(9), "2.5"},
		{"a value an index or an if picks: canonical", "picked", edit.Source("{ x: 3, y: 4 }"), "{ x: 2.5, y: 3 }"},
		{"a value a match arm gives: canonical", "toned", edit.Source("{ x: 3, y: 3 }"), "{ x: 2.5, y: 2.0 }"},
	} {
		p, _, ok := applyIn(t, c.name, spellFS(), undoView{}, []edit.Operation{setAt(c.path, c.v)})
		if !ok {
			continue
		}
		if len(p.Undo) != 1 || p.Undo[0].Value != c.want {
			t.Errorf("%s: Undo %+v, want one Set to Source %q", c.name, p.Undo, c.want)
		}
	}
}

// DECISIONS 337, API.md E22, E23: a removed entry comes back through AddEntry and Move, and the
// literal fields its restore writes keep their spelling (values, not layout).
func TestUndoRemovedEntrySpelling(t *testing.T) {
	p, f1, ok := applyIn(t, "remove", spellFS(), undoView{}, []edit.Operation{{Kind: edit.OpRemove, Path: "skills.slash"}})
	if !ok {
		return
	}
	_, f2, ok := applyIn(t, "remove, Undo", f1, undoView{}, p.Undo)
	if !ok {
		return
	}
	sameValues(t, "remove, Undo", spellFS(), f2)
	got := string(f2["law/d/d.canon"].Data)
	for _, tok := range []string{"1.0", "1e3", "0.50", "0x1F", "1_000", `"a\tb\u{41}\"c"`, "2.50", "-1.0"} {
		if !strings.Contains(got, tok) {
			t.Errorf("remove, Undo: %s is not written back as written:\n%s", tok, got)
		}
	}
}

// DECISIONS 337, API.md E22, W11: under an edit layer, active or not, the Undo still restores the
// layer's own lines, a line as it was, spelled as written, or a Reset where there was none.
func TestUndoLayerLineSpelling(t *testing.T) {
	dev := "package d\nlayer dev\n\namend skills {\n  slash.rate: 0.750\n  bash.mask: 0x0F\n}\n"
	fs := spellFS()
	fs["law/d/dev.layer.canon"] = file(dev)
	files := []string{"law/d/d.canon", "law/d/dev.layer.canon"}
	for _, layers := range [][]string{{"dev"}, nil} {
		v := undoView{layers, "dev"}
		for _, c := range []struct {
			name string
			ops  []edit.Operation
		}{
			{"a line spelled as written", []edit.Operation{setAt("skills.slash.rate", edit.Float(0.5))}},
			{"two lines", []edit.Operation{setAt("skills.slash.rate", edit.Float(0.5)), setAt("skills.bash.mask", edit.Int(1))}},
			{"no line: a Reset", []edit.Operation{setAt("skills.slash.mul", edit.Source("4.0"))}},
		} {
			undoIn(t, undoSpec{name: c.name, fsys: fs, view: v, views: [][]string{{"dev"}, nil}, files: files, ops: c.ops})
		}
	}
}

// API.md E22, W11 (DECISIONS 273): the edit layer's lines are verified by value, so a spelling
// the formatter keeps (parentheses, a multiline string) never fails the Undo with ErrInternal.
func TestUndoLayerLineKeptSpelling(t *testing.T) {
	both, active := [][]string{{"dev"}, nil}, [][]string{{"dev"}}
	for _, c := range []struct {
		name, lines string
		views       [][]string
		ops         []edit.Operation
	}{
		{"a number in parentheses, Set", "  slash.rate: (0.750)\n", both, []edit.Operation{setAt("skills.slash.rate", edit.Float(0.5))}},
		{"a number in parentheses, Reset", "  slash.rate: (0.750)\n", both, []edit.Operation{{Kind: edit.OpReset, Path: "skills.slash.rate"}}},
		{"a raw string in parentheses", "  bash.label: (r\"q\\w\")\n", both, []edit.Operation{{Kind: edit.OpReset, Path: "skills.bash.label"}}},
		{"a multiline string", "  bash.label: \"\"\"\n    two\n    lines\n    \"\"\"\n", both,
			[]edit.Operation{setAt("skills.bash.label", edit.Str("z"))}},
		{"an element in parentheses removed", "  slash.steps: [1.0, (2.0)]\n", active,
			[]edit.Operation{{Kind: edit.OpRemove, Path: "skills.slash.steps[1]"}}},
		{"two lines and a line added", "  slash.rate: (0.750)\n  bash.mask: (0x0F)\n", both, []edit.Operation{
			setAt("skills.slash.rate", edit.Float(0.5)), {Kind: edit.OpReset, Path: "skills.bash.mask"}, setAt("skills.slash.mul", edit.Float(3))}},
	} {
		fs := spellFS()
		fs["law/d/dev.layer.canon"] = file("package d\nlayer dev\n\namend skills {\n" + c.lines + "}\n")
		for _, layers := range c.views {
			undoIn(t, undoSpec{name: c.name, fsys: fs, view: undoView{layers, "dev"}, views: c.views,
				files: []string{"law/d/d.canon"}, ops: c.ops})
		}
	}
}
