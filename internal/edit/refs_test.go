package edit_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// refsR is a package naming the entry statuses.done, and the member Tone.loud, in every place
// API.md R7 lists.
const refsR = `/// R.
package r

/// A tone.
enum Tone { calm, loud = "LOUD" }

/// A status.
record Status {
  /// Its tone.
  tone: Tone = calm
  /// Where it goes.
  next: [ref statuses] = []
  /// A fallback.
  fallback: ref statuses = done

  check not (tone == loud and next.contains(done)) else "done follows a loud status"
}

/// Statuses.
let statuses: table Status = {
  open { next: [done] }
  done { tone: loud }
}

/// Weights.
let weights: {ref statuses: Int} = { open: 1, done: 2 }

/// Tones by name.
let byTone: {Tone: Int} = { loud: 1 }

/// Loaded statuses.
let loaded: [ref statuses] = load("loaded.json")

/// Read through a name.
let viaName: Status = statuses.open

/// Chosen.
let chosen: Status = { tone: calm }

/// Whether a status is done.
fn isDone(s: ref statuses) -> Bool {
  return s == done or s == later
}

check statuses.done.tone == loud else "done is loud"

view Status {
  show "Done" "{statuses.done.tone}"
}

/// A configuration.
record Conf {
  /// The first status.
  first: ref statuses
}

/// Loaded, then amended by dev.
let conf: Conf = load("s.json")

/// A qualified member.
let q: Tone = Tone.loud

/// A status code picks.
fn pick() -> ref statuses {
  return done
}

/// Picked.
let picked: ref statuses = pick()

/// An entry read as a ref.
let pd: ref statuses = statuses.done

/// Entries read as refs.
let pds: [ref statuses] = [statuses.done]

test "done is loud" {
  expect statuses.done.tone == loud
}
`

// refsDev amends a loaded value and a base literal holding the target.
const refsDev = `package r
layer dev

amend chosen {
  fallback: done
}

amend conf {
  first: open
}

amend statuses {
  open.next: [open]
}
`

var refsLaw = mapFS{
	"law/project.canon":     file("project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n"),
	"law/r/r.canon":         file(refsR),
	"law/r/more.canon":      file("package r\n\n/// Later.\nentry statuses.later { next: [done] }\n"),
	"law/r/loaded.json":     file("[\"open\", \"done\"]\n"),
	"law/r/s.json":          file("{\"first\": \"done\"}\n"),
	"law/r/dev.layer.canon": file(refsDev),
	"law/r/r.fr.canon":      file("package r\ntranslation fr\n\nTone.loud \"Fort\"\n"),
	"law/q/q.canon":         file("/// Q.\npackage q\n\nimport r { statuses }\n\n/// Elsewhere.\nlet far: [ref statuses] = [done]\n"),
}

// refsFixture is refsLaw with its dev layer active.
func refsFixture(t *testing.T) fixture {
	t.Helper()
	p, err := build.Open(refsLaw, "/law", build.Options{Layers: []string{"dev"}})
	if err != nil {
		t.Fatal(err)
	}
	return analyze(t, p, []string{"r", "q"}, true)
}

var refKinds = map[edit.RefKind]string{
	edit.RefValue: "value", edit.RefKey: "key", edit.RefCode: "code", edit.RefView: "view", edit.RefCheck: "check", edit.RefLayer: "layer",
}

// refLines are refs as `<file>:<line> <kind> <package>[:<path>] <text>`.
func refLines(f fixture, refs []edit.Ref) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		where := r.Package
		if r.Path != "" {
			where += ":" + r.Path
		}
		line, _ := f.files.Position(r.Span.File, r.Span.Start)
		out[i] = fmt.Sprintf("%s:%d %s %s %s", f.files.Path(r.Span.File), line, refKinds[r.Kind], where, f.text(r.Span))
	}
	return out
}

// refsCase is a path and its references, as refLines writes them.
type refsCase struct {
	path string
	want []string
}

func checkRefs(t *testing.T, f fixture, cases []refsCase) {
	t.Helper()
	for _, c := range cases {
		refs, err := f.Refs(context.Background(), resolve(t, f, c.path))
		if err != nil {
			t.Fatal(err)
		}
		if got := refLines(f, refs); !slices.Equal(got, c.want) {
			t.Errorf("Refs(%s) =\n%s\nwant\n%s", c.path, strings.Join(got, "\n"), strings.Join(c.want, "\n"))
		}
	}
}

// API.md R7 (log-2026-09-29 M4 U4a): every kind of reference, in every loaded package, listed
// once; a value read through a name (viaName) at its source's path; a default as code; a base
// literal and data an active layer amends as values; a computed ref as value and code (API.md R8).
func TestRefsListEveryReference(t *testing.T) {
	checkRefs(t, refsFixture(t), []refsCase{
		{"r:statuses.done", []string{
			"q/q.canon:7 value q:far[0] done",
			"r/dev.layer.canon:5 layer r done",
			`r/loaded.json:1 value r:loaded[1] "done"`,
			"r/more.canon:4 value r:statuses.later.next[0] done",
			"r/r.canon:14 code r done",
			"r/r.canon:16 check r done",
			"r/r.canon:21 value r:statuses.open.next[0] done",
			"r/r.canon:22 value r:pd done { tone: loud }", // converted from the entry: computed
			"r/r.canon:22 value r:pds[0] done { tone: loud }",
			"r/r.canon:26 key r:weights[done] done",
			"r/r.canon:42 code r done",
			"r/r.canon:45 check r done",
			"r/r.canon:48 view r done",
			"r/r.canon:65 value r:picked done", // computed in pick(): not editable, E12
			"r/r.canon:65 code r done",
			"r/r.canon:72 code r done", // the name pd is converted from, not hidden
			"r/r.canon:75 code r done",
			"r/r.canon:78 code r done",             // a test declaration
			`r/s.json:1 value r:conf.first "done"`, // held in data dev amends
		}},
		// An entry declared apart, named in code (TYPES.md §4.1 static keys).
		{"r:statuses.later", []string{"r/r.canon:42 code r later"}},
		// API.md P7a: an enum member's references.
		{"r:Tone.loud", []string{
			"r/r.canon:16 check r loud",
			"r/r.canon:22 value r:statuses.done.tone loud",
			"r/r.canon:29 key r:byTone[loud] loud",
			"r/r.canon:45 check r loud",
			"r/r.canon:61 value r:q Tone.loud", // once: the qualified name is the value's own
			"r/r.canon:78 code r loud",
			"r/r.fr.canon:4 view r loud", // a translation
		}},
	})
}

// API.md R7: an entry declared apart, a keyed-list element, a ref key of a dependent map, a
// key read in code.
func TestRefsOfEntriesAndElements(t *testing.T) {
	checkRefs(t, lawFixture(t, nil, "a"), []refsCase{
		{"a:entries.one", []string{"a/a.canon:90 value a:links[one].to one", "a/a.canon:110 key a:perEntry[one] one"}},
		{"a:entries.two", []string{"a/a.canon:110 key a:perEntry[two] two"}},
		{"a:items[a]", []string{"a/a.canon:100 code a a"}},
	})
}

// withBad is refsLaw with a file of package r holding extra, analyzed even though it fails.
func withBad(t *testing.T, extra string) fixture {
	t.Helper()
	fsys := mapFS{}
	maps.Copy(fsys, refsLaw)
	fsys["law/r/bad.canon"] = file("package r\n\n" + extra)
	p, err := build.Open(fsys, "/law", build.Options{Layers: []string{"dev"}})
	if err != nil {
		t.Fatal(err)
	}
	return analyze(t, p, []string{"r", "q"}, false)
}

// API.md R7 (log-2026-09-29 M4 U4a): a let that could hold the target but has no value fails
// Refs, naming it, for a partial list never looks complete; one whose type holds none is ignored.
func TestRefsFailOnMissingValues(t *testing.T) {
	ctx := context.Background()
	f := withBad(t, "/// A pool.\nlet pool: [ref statuses] = [done]\n\n/// Broken.\nlet broken: ref statuses = pool[5]\n")
	_, err := f.Refs(ctx, resolve(t, f, "r:statuses.done"))
	var pe *edit.PathError
	if !errors.As(err, &pe) || !errors.Is(err, edit.ErrNoValue) || pe.Root.Name != "broken" {
		t.Errorf("Refs with r:broken poisoned = %v, want ErrNoValue naming broken", err)
	}
	f = withBad(t, "/// Numbers.\nlet nums: [Int] = [1]\n\n/// Broken.\nlet count: Int = nums[5]\n")
	if _, err := f.Refs(ctx, resolve(t, f, "r:statuses.done")); err != nil {
		t.Errorf("Refs with an Int poisoned: %v, want the list", err)
	}
}

// log-2026-09-29 M4 U4a: a broken let, typed Error, holds no ref value, so it fails no Refs.
func TestRefsIgnoreBrokenLets(t *testing.T) {
	f := withBad(t, "/// Broken.\nlet bad = nosuchname\n")
	refs, err := f.Refs(context.Background(), resolve(t, f, "r:statuses.done"))
	if err != nil || len(refs) == 0 {
		t.Errorf("Refs with a broken let = %d refs, %v, want the list", len(refs), err)
	}
}

// scaleR is a let of records holding refs, which the layer big replaces whole.
const scaleR = `/// R.
package r

/// A status.
record Status {
  /// A label.
  label: String = ""
}

/// Statuses.
let statuses: table Status = {
  open {}
  done {}
}

/// A record.
record Rec {
  /// Its status.
  s: ref statuses
}

/// A box.
record Box {
  /// Records.
  recs: [Rec]
}

/// The box.
let box: Box = { recs: [%s] }
`

// scaleVisits is how many values Refs visits with n records replaced by a layer.
func scaleVisits(t *testing.T, n int) int {
	t.Helper()
	recs := strings.TrimSuffix(strings.Repeat("{ s: done }, ", n), ", ")
	layer := "package r\nlayer big\n\namend box {\n  recs: [" + strings.ReplaceAll(recs, "done", "open") + "]\n}\n"
	fsys := mapFS{
		"law/project.canon":     file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/r/r.canon":         file(fmt.Sprintf(scaleR, recs)),
		"law/r/big.layer.canon": file(layer),
	}
	p, err := build.Open(fsys, "/law", build.Options{Layers: []string{"big"}})
	if err != nil {
		t.Fatal(err)
	}
	f := analyze(t, p, []string{"r"}, true)
	target := resolve(t, f, "r:statuses.done")
	visits, err := edit.RefVisits(context.Background(), f.Snapshot, target)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := f.Refs(context.Background(), target)
	if err != nil || len(refs) != n {
		t.Fatalf("Refs with %d replaced records = %d refs, %v, want each", n, len(refs), err)
	}
	return visits
}

// log-2026-09-29 M4 U4a: values a layer replaced are walked once each: the walk grows with the
// records, not with their square.
func TestRefsWalkIsLinear(t *testing.T) {
	small, large := scaleVisits(t, 100), scaleVisits(t, 400)
	if large > small*6 {
		t.Errorf("Refs visits %d values for 100 records, %d for 400: not linear", small, large)
	}
}

// API.md R7: a path that names no entry, keyed-list element or member is ErrBadOp.
func TestRefsRefuseOtherPaths(t *testing.T) {
	f := refsFixture(t)
	for _, path := range []string{"r:statuses", "r:statuses.done.tone", "r:weights[done]", "r:chosen"} {
		_, err := f.Refs(context.Background(), resolve(t, f, path))
		if !errors.Is(err, edit.ErrBadOp) {
			t.Errorf("Refs(%s) = %v, want ErrBadOp", path, err)
		}
	}
}
