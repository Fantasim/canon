package ir_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	findingsFile = "findings.txt"
	selectedFile = "selected.txt" // the packages a case builds, space-separated; all when absent
)

// reFindingCode is a rendered finding's code (diag.Render's `error[Exxxx]`/`warning[Wxxxx]`),
// not a code merely mentioned in a message's text.
var reFindingCode = regexp.MustCompile(`(?:error|warning)\[([EW][0-9]{4})\]`)

// IMPLEMENTATION-PLAN.md §7.2: each emit rule's txtar case checks cleanly and fails only its stage-E rule; values come from its `<pkg>.<name>.json` files.
func TestFindings(t *testing.T) {
	onlyItsCode(t, "testdata/findings/*.txtar")
}

// TestCascades is DECISIONS 213 (no stage-E finding on a broken declaration or a refused emit's mode, one E8011 per refused type name; 209: a broken check breaks its record) and one finding per refusal in stage E: each case's program has one error, the finding its file names, and stage E adds nothing to it.
func TestCascades(t *testing.T) {
	onlyItsCode(t, "testdata/cascades/*.txtar")
}

// onlyItsCode builds every case of glob (the packages of its selected.txt, else all) and requires the findings to be the code its file name starts with, and nothing else.
func onlyItsCode(t *testing.T, glob string) {
	t.Helper()
	golden.Run(t, glob, func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		var selected []string
		for _, f := range c.Archive.Files {
			switch f.Name {
			case findingsFile:
			case selectedFile:
				selected = strings.Fields(string(f.Data))
			default:
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		w.build(t, selected...)
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

// TestE8007OnlyForUnmappedRoot is decision 194's review: an unselected dependency's go emit whose out does not resolve at all is not E8007 ("under no mapped root"); only a resolved directory outside every go_module root is.
func TestE8007OnlyForUnmappedRoot(t *testing.T) {
	w := newWorld(t)
	w.add(t, "b/b.canon", []byte(`package b

/// A colour.
enum Tone { red, blue }

emit go { out: "@nosuchroot/b", package: "b" }
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
	if out := w.findings(t); strings.Contains(out, "["+string(diag.E8007.Def().Code)+"]") {
		t.Errorf("an out that does not resolve is not %s:\n%s", string(diag.E8007.Def().Code), out)
	}
}

// TestGoNamePlanIsExact is CODEGEN.md §3.3, §3.5 and decisions 194, 203: E8005 reads the names gen/go declares, so an enum member's override that is its whole constant (B next to ToneB), a table the go emit leaves out of `values` (no container Potion) and and an override that renames the storage too (a field's FooBarAlt and fooBarAlt next to fooBar; a value's getOther next to fooBar; a package fn's otherTable next to aBTable; meta/decisions/log-2026-09-24.md, IR round 2 review) are clean.
func TestGoNamePlanIsExact(t *testing.T) {
	for name, src := range map[string]string{ //canon:unordered each case alone
		"value override renames storage": `package a

/// One flag.
let fooBar: Bool = true

/// The other flag.
@go(name: "GetOther")
let foo_bar: Bool = false

emit go { out: "@features/a", package: "a" }
`,
		"fn override renames its table": `package a

/// One answer.
export fn aB() -> Int { return 1 }

/// The other answer.
@go(name: "Other")
export fn a_b() -> Int { return 2 }

emit go { out: "@features/a", package: "a" }
`,
		"override renames storage": `package a

/// A rule.
record Rule {
  /// One score.
  fooBar: Int
  /// The other score.
  foo_bar: Int @go(name: "FooBarAlt")
}

emit go { out: "@features/a", package: "a" }
`,
		"enum override": `package a

/// A tone.
enum Tone { a @go(name: "B"), b }

emit go { out: "@features/a", package: "a" }
`,
		"unselected table": `package a

/// A potion.
record Potion {
  /// Its heal.
  heal: Int
}

/// The potions.
let potion: table Potion = {
  small { heal: 1 }
}

/// Whether potions are on.
let on: Bool = true

emit go { out: "@features/a", package: "a", values: [on] }
`,
	} {
		w := newWorld(t)
		w.add(t, "a/a.canon", []byte(src))
		w.calls = w.fixtureCalls
		w.build(t)
		if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
			t.Errorf("%s must be clean:\n%s", name, out)
		}
	}
}
