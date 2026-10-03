package check_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// depFixture reaches branch fields through dependent fields in a `@files` template: a name both
// branches declare, one only one branch declares, and one past a nested application.
var depFixture = [][2]string{
	{"d/d.canon", `package d

enum Shape { pt, seg, far }

enum Size { small, large }

record Pos {
  y: Int
}

record Pt {
  x: Int
}

record Seg {
  x: Int
  len: Int
}

record Far {
  at: Pos
}

type Deep(s: Size) = match s {
  small => Int
  large => Far
}

type Where(s: Shape, z: Size) = match s {
  pt => Pt
  seg => Seg
  far => Deep(z)
}

record Spot {
  shape: Shape
  size: Size
  spot: Where(shape, size)
}

@files("s/{spot.x}/{spot.len}/{spot.at.y}/{id}.canon")
let spots: table Spot = {}
`},
}

// GRAMMAR.md §8 `@files`, API.md E33, IMPLEMENTATION-PLAN §4.7, DECISIONS 277, 280: branches are candidates.
func TestOccurrencesThroughDependentFields(t *testing.T) {
	prog, files, out := checkSet(t, depFixture)
	if strings.Contains(out, "error[") {
		t.Fatalf("findings:\n%s", out)
	}
	wantOccurrences(t, prog, files, depCases)
}

var depCases = []occCase{
	{"Pt.x: ambiguous, Seg declares x too", declAt{"d/d.canon", "x", 1}, []string{
		"d/d.canon:12:3 decl code x", "d/d.canon:41:17 files-var code x ambiguous",
	}},
	{"Seg.x: ambiguous, Pt declares x too", declAt{"d/d.canon", "x", 2}, []string{
		"d/d.canon:16:3 decl code x", "d/d.canon:41:17 files-var code x ambiguous",
	}},
	{"Seg.len: one branch declares it", declAt{"d/d.canon", "len", 1}, []string{
		"d/d.canon:17:3 decl code len", "d/d.canon:41:26 files-var code len",
	}},
	{"Far.at: through the nested application Deep(z)", declAt{"d/d.canon", "at", 1}, []string{
		"d/d.canon:21:3 decl code at", "d/d.canon:41:37 files-var code at",
	}},
	{"Pos.y: past the nested application", declAt{"d/d.canon", "y", 1}, []string{
		"d/d.canon:8:3 decl code y", "d/d.canon:41:40 files-var code y",
	}},
	{"spot: the dependent field heading each path", declAt{"d/d.canon", "spot", 1}, []string{
		"d/d.canon:38:3 decl code spot", "d/d.canon:41:12 files-var code spot",
		"d/d.canon:41:21 files-var code spot", "d/d.canon:41:32 files-var code spot",
	}},
}

// DECISIONS 280: an amend path through a dependent field is E1905, and its segment is not
// indexed under a branch's field.
func TestAmendThroughDependentFieldNotIndexed(t *testing.T) {
	files := [][2]string{
		{depFixture[0][0], depFixture[0][1] + "\nlet one: Spot = { shape: seg, size: small, spot: { x: 1, len: 2 } }\n"},
		{"d/dev.layer.canon", "package d\nlayer dev\n\namend one {\n  spot.len: 3\n}\n"},
	}
	prog, parsed, out := checkSet(t, files)
	if want := "[" + string(diag.E1905.Def().Code) + "]"; !strings.Contains(out, want) {
		t.Fatalf("want %s, got:\n%s", want, out)
	}
	wantOccurrences(t, prog, parsed, []occCase{
		{"Seg.len: the template's name, not the amend segment", declAt{"d/d.canon", "len", 1}, []string{
			"d/d.canon:17:3 decl code len", "d/d.canon:41:26 files-var code len",
		}},
	})
}
