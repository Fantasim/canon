package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// zoo keeps its types in one file and its data in lets of another, a table, a keyed list and a
// plain let, which a third file's entries and lets read.
var zoo = map[string]string{
	"zoo/types.canon": `package zoo

/// An animal.
record Animal {
  /// Its name.
  name: String(1..)
  /// Its legs.
  legs: Int(0..=LEGS)
}

/// A pen.
record Pen {
  /// Its code.
  code: Int
  /// Its size.
  size: Int(1..=9)
}

/// The most legs.
const LEGS = 8

/// The animals.
let animals: table Animal = {}
`,
	"zoo/data.canon": `package zoo

/// The pens, by code.
let pens: [Pen] keyed by code = [{ code: 1, size: 2 }, { code: 2, size: 3 }]

/// Other animals.
let others: table Animal = {
  ant { name: "Ant", legs: 6 }
  eel { name: "Eel", legs: 0 }
}

/// A count.
let count: Int(0..=99) = 2 + others.ant.legs

entry animals.cat { name: "Cat", legs: 4 }
`,
	"zoo/use.canon": `package zoo

entry animals.dog { name: "Dog", legs: 4 }

entry pens.3 { size: 4 }

entry pens.1 { size: 5 }

/// The ant's legs, twice.
let twice: Int = others.ant.legs * 2 + count
`,
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: let values re-check as cold; keys, annotation, parse error refuse.
func TestRecheckLets(t *testing.T) {
	steps := []step{
		{"zoo/data.canon", "legs: 6 }", "legs: 5 }", true},
		{"zoo/data.canon", "size: 3 }", "size: 4 }", true},
		{"zoo/data.canon", "= 2 + others", "= 3 + others", true},
		{"zoo/data.canon", "legs: 0 }", "legs: 12 }", true},
		{"zoo/data.canon", "legs: 12 }", `legs: "none" }`, true},
		{"zoo/data.canon", `legs: "none" }`, "legs: 0 }", true},
		{"zoo/data.canon", "/// Other animals.", "/// More animals.", true},
		{"zoo/data.canon", `name: "Cat"`, `name: "Kit"`, true},
		{"zoo/use.canon", "* 2 +", "* 3 +", true},
		{"zoo/data.canon", "= 3 + others", "= 3 + 1 + others", true},
		{"zoo/data.canon", "eel {", "cod {", false},
		{"zoo/data.canon", "code: 2, size", "code: 5, size", false},
		{"zoo/data.canon", "let count: Int(0..=99)", "let count: Int(0..=98)", false},
		{"zoo/data.canon", "= 3 + 1 + others", "= 3 + + others", false},
	}
	w := newWorld(t, zoo)
	_, s, _ := w.session()
	for _, st := range steps {
		s = w.step(s, st)
	}
}

// qualified imports another package and names it in a let's type, the file's doc comment first.
var qualified = map[string]string{
	"other/o.canon":  "package other\n\n/// A thing.\nrecord Thing {\n  /// N.\n  n: Int\n}\n",
	"zoo/data.canon": "/// Zoo.\npackage zoo\n\nimport other\n\n/// T.\nlet t: other.Thing = { n: 1 }\n",
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (identities, log-2026-09-30 P15-r (b)): one object per import.
func TestRecheckImportIdentity(t *testing.T) {
	w := newWorld(t, qualified)
	_, s, _ := w.session()
	s = w.step(s, step{"zoo/data.canon", "/// Zoo.", "/// Zoos.", true})
	s = w.step(s, step{"zoo/data.canon", "n: 1 }", "n: 2 }", true})
	prog, _, _, ok := w.recheck(s, w.edit("zoo/data.canon", "/// Zoos.", "/// Zoo."))
	f := w.files["zoo/data.canon"]
	var qual *syntax.Ident
	syntax.Inspect(f.Decls[0].(*syntax.LetDecl).Type, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.Ident); ok && id.Name == "other" && qual == nil {
			qual = id
		}
		return true
	})
	if !ok || qual == nil {
		t.Fatalf("Recheck %v, qualifier found %v", ok, qual != nil)
	}
	imp := f.Imports[0].Path.Parts[0]
	if bound, named := prog.Info.NameUses[imp], prog.Info.NameUses[qual]; bound != named {
		t.Errorf("the import binds %p, the let's type names %p", bound, named)
	}
}
