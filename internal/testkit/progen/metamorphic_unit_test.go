package progen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	metaPath = "m/m.canon"
	metaPkg  = "m"
)

// metaFixture is a small package: documented declarations, a multi-line string, a `local` fn
// whose parameter shadows a `local` const's name, a stable table and two locals free to move.
const metaFixture = `package m
/// T.
enum T { a, b }
/// FOO.
local const FOO = 1
/// bar.
local let bar: Int = FOO + 2
local let text: String = """
  x

  y
  """
/// R.
record R {
  /// n.
  n: Int
}
local let rows: stable table R = {
  k1 { n: 1 }
}
local const N = 3
local fn twice(N: Int) -> Int { return N * 2 }
`

// metaProject is a project whose map entries a reorder may swap (GRAMMAR.md §7.1).
const metaProject = "project p {\n  canon: \"0.1\"\n  budget: 200_000\n  roots {\n    a: \"../a\"\n    b: \"../b\"\n  }\n}\n"

func metaTarget(path, src string) target {
	tg := target{pkg: metaPkg, path: path, src: []byte(src), file: parse(path, []byte(src))}
	tg.all = &[]target{tg}
	return tg
}

// metaFixtureProject is the fixture package in a project.
func metaFixtureProject() *progen.Project {
	p := progen.NewProject()
	p.Set(projectFile, []byte(metaProject))
	p.Set(metaPath, []byte(metaFixture))
	return p
}

// DECISIONS 200 item 4: every relation cites the rule it rests on, finds sites in the fixture,
// and each of them changes no finding and no output (the property itself, on a small file).
func TestMetaRelationsHold(t *testing.T) {
	base := runMetaBuild(metaFixtureProject(), metaPkg)
	for _, op := range metaOperators() {
		tg := metaTarget(metaPath, metaFixture)
		sites := op.sites(tg)
		if op.rule == "" || len(sites) == 0 {
			t.Fatalf("%s: rule %q, %d sites in the fixture", op.name, op.rule, len(sites))
		}
		for _, s := range sites {
			m, err := progen.Site{Path: tg.path, Edits: s.edits}.Mutate(metaFixtureProject())
			if err != nil {
				t.Fatalf("%s %s: %v", op.name, s.desc, err)
			}
			if v := compareMeta(base, runMetaBuild(m.Project, metaPkg)); v.Kind != "" {
				t.Errorf("%s %s: %s", op.name, s.desc, v.Text)
			}
		}
	}
}

// GRAMMAR.md §9.1: no blank line after a doc block, no insertion inside a multi-line string.
func TestTriviaSites(t *testing.T) {
	tg := metaTarget(metaPath, metaFixture)
	docEnd := strings.Index(metaFixture, "/// T.\n") + len("/// T.\n")
	inString := strings.Index(metaFixture, "  x\n\n") + len("  x\n")
	blank, comment := offsets(blankSites(tg)), offsets(commentSites(tg))
	switch {
	case blank[docEnd]:
		t.Error("a blank line after a doc block")
	case !comment[docEnd]:
		t.Error("no comment line between a doc block and its item")
	case blank[inString] || comment[inString]:
		t.Error("an insertion inside a multi-line string")
	case len(blank) == 0 || len(comment) <= len(blank):
		t.Errorf("%d blank and %d comment sites", len(blank), len(comment))
	}
}

func offsets(sites []metaSite) map[int]bool {
	out := map[int]bool{}
	for _, s := range sites {
		out[s.edits[0].Start] = true
	}
	return out
}

// The rename is syntactic, giving up on any non-name use of the spelling: not N, not rows (LOCK.md §1).
func TestRenameSites(t *testing.T) {
	got := map[string]int{}
	for _, s := range renameSites(metaTarget(metaPath, metaFixture)) {
		got[s.desc] = len(s.edits)
	}
	want := map[string]int{"rename FOO to FOO_2": 2, "rename bar to barZz": 1, "rename text to textZz": 1, "rename twice to twiceZz": 1}
	for desc, n := range want {
		if got[desc] != n {
			t.Errorf("%s: %d edits, want %d (all: %v)", desc, got[desc], n, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("sites %v, want %v", got, want)
	}
}

// The review call "reorder only local declarations and project keys": bar reads FOO, so the
// two never swap; the project's roots entries do.
func TestReorderSites(t *testing.T) {
	for _, s := range reorderSites(metaTarget(metaPath, metaFixture)) {
		if s.desc == "swap FOO and bar" {
			t.Errorf("%s: bar reads FOO", s.desc)
		}
	}
	sites := reorderSites(metaTarget(projectFile, metaProject))
	if len(sites) != 1 || sites[0].desc != "swap a and b" {
		t.Fatalf("project sites %v", sites)
	}
}

// A metamorphic signature names the findings gained and lost and the output that changed.
func TestMetaSignatures(t *testing.T) {
	w, e := diag.W1002.Def().Code, diag.E2102.Def().Code
	base := []progen.Finding{{Code: w, Message: "m1"}}
	out := []progen.Finding{{Code: e, Message: "m2"}}
	if got, want := findingDiff(base, out), "+"+string(e)+shapeSep+"-"+string(w); got != want {
		t.Errorf("findingDiff = %q, want %q", got, want)
	}
	a := []build.Output{{Path: "@o/a.go", Content: []byte("x")}}
	b := []build.Output{{Path: "@o/a.go", Content: []byte("y")}, {Path: "@o/b.go"}}
	if got := diffOutputs(a, b); got != "changed @o/a.go" {
		t.Errorf("diffOutputs = %q", got)
	}
	if got := diffOutputs(a, a[:0]); got != "dropped @o/a.go" {
		t.Errorf("diffOutputs = %q", got)
	}
}

// The review call "self-contained archives": a kept metamorphic case replays from its own files,
// its baseline built from the archive, never from the examples.
func TestMetaArchiveReplays(t *testing.T) {
	p := metaFixtureProject()
	tg := metaTarget(metaPath, metaFixture)
	broken := strings.Replace(metaFixture, "FOO + 2", "FOO + zz", 1)
	variant := p.Clone()
	variant.Set(metaPath, []byte(broken))
	v := compareMeta(runMetaBuild(p, metaPkg), runMetaBuild(variant, metaPkg))
	if v.Kind != kindMetaFindings {
		t.Fatalf("the broken variant: %+v", v)
	}
	mc := metaCase{op: metaOperators()[0], tg: tg, site: metaSite{desc: "test"}}
	arch := metaArchive(mc, trimMeta(p, tg, variant, v.Sig), variant, v)
	name := filepath.Join(t.TempDir(), "meta.txtar")
	if err := os.WriteFile(name, arch.Format(), 0o644); err != nil {
		t.Fatal(err)
	}
	back, err := progen.ReadCounterexample(name)
	if err != nil {
		t.Fatal(err)
	}
	if got := replayMeta(back); sigKey(got.Sig) != sigKey(v.Sig) {
		t.Errorf("replayed %q, kept %q", got.Sig, v.Sig)
	}
	if _, ok := back.Files.Get(variantDir + metaPath); !ok {
		t.Error("the archive holds no variant file")
	}
}

// Review call on DECISIONS 200 item 4 ("whitespace" covers spacing within a line): spacing sites
// widen spaces and indent lines, never inside the multi-line string, never with a line break.
func TestSpacingSites(t *testing.T) {
	tg := metaTarget(metaPath, metaFixture)
	open := strings.Index(metaFixture, `"""`)
	closing := strings.LastIndex(metaFixture, `"""`)
	sites := spacingSites(tg)
	var widen, indent int
	for _, s := range sites {
		e := s.edits[0]
		switch {
		case e.Start > open && e.Start <= closing:
			t.Errorf("%s: inside the multi-line string", s.desc)
		case strings.Contains(e.Text, "\n"):
			t.Errorf("%s: a line break", s.desc)
		case e.Text == metaSpace:
			widen++
		default:
			indent++
		}
	}
	if widen == 0 || indent == 0 {
		t.Errorf("%d wider spaces and %d indentations", widen, indent)
	}
}
