package ir_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const findingsFile = "findings.txt"

// reFindingCode is a rendered finding's code (diag.Render's `error[Exxxx]`/`warning[Wxxxx]`),
// not a code merely mentioned in a message's text.
var reFindingCode = regexp.MustCompile(`(?:error|warning)\[([EW][0-9]{4})\]`)

// IMPLEMENTATION-PLAN.md §7.2: each emit rule's txtar case checks cleanly and fails only its stage-E rule; values come from its `<pkg>.<name>.json` files.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != findingsFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		w.build(t)
		out := w.findings(t)
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		for _, m := range reFindingCode.FindAllStringSubmatch(out, -1) {
			if m[1] != code {
				t.Errorf("%s also produces %s, not just %s:\n%s", c.Path, m[1], code, out)
			}
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}

// TestE8007ForUnselectedDependency is IMPLEMENTATION-PLAN.md §4.5: an unselected dependency's own go emit is under no mapped root, and it never checks itself, so the importer reports it.
func TestE8007ForUnselectedDependency(t *testing.T) {
	w := newWorld(t)
	w.add(t, "b/b.canon", []byte(`package b

/// A colour.
enum Tone { red, blue }

emit go { out: "out/go/", package: "b" }
`))
	w.add(t, "a/a.canon", []byte(`package a

import b { Tone }

/// A badge.
record Badge {
  /// Its colour.
  tone: Tone
}

emit go { out: "@features/a", package: "a" }
`))
	w.calls = w.fixtureCalls
	w.build(t, "a")
	out := w.findings(t)
	if !strings.Contains(out, "["+string(diag.E8007.Def().Code)+"]") || !strings.Contains(out, "a/a.canon") {
		t.Errorf("the importer must report %s for its unselected dependency's own emit:\n%s", string(diag.E8007.Def().Code), out)
	}
}

// TestE8101SkipsTypesMode is decision 194: a TS `types` emit writes no value data, so an out-of-range integer there is not E8101.
func TestE8101SkipsTypesMode(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A counter.
record Counter {
  /// Its total.
  total: Int
}

/// The counter.
let counter: Counter = { total: 9_007_199_254_740_993 }

emit ts { out: "out/a.ts", mode: types }
`))
	w.add(t, "a.counter.json", []byte(`{"total": 9007199254740993}`))
	w.calls = w.fixtureCalls
	w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Errorf("a types-mode emit must not report %s for a value's own data:\n%s", string(diag.E8101.Def().Code), out)
	}
}

// TestDefineWithoutBakedGoIsClean is decision 180 (only baked go refuses a define record) and decision 37 (check fails wherever build would, no more): a cpp-only emit of the same package gets no E8012.
func TestDefineWithoutBakedGoIsClean(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A rule.
record Rule {
  /// A define.
  it: Define
}

/// The rule.
let rule: Rule = { it: { value: 3 } }

emit cpp { out: "@features/a" }
`))
	w.calls = w.fixtureCalls
	w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Errorf("a define record with no baked go emit must not be %s:\n%s", string(diag.E8012.Def().Code), out)
	}
}
