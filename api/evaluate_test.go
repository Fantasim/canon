package canon_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

var update = flag.Bool("update", false, "rewrite api/testdata/evaluate instead of comparing")

// evalLaw has views with a translated title, a subtitle, a `when`, a `show` line, a dependent
// field, a list of elements with equal titles and a failing check; b imports a, with its own error.
var evalLaw = map[string]string{
	"project.canon": "project demo {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n",
	"a/a.canon": `/// A.
package a

/// A goal.
enum Goal { kill, collect }

/// What a goal aims at.
type Target(g: Goal) = match g {
  kill => String
  collect => Int
}

/// A part.
record Part {
  /// Its label.
  label: String
  /// Its size.
  size: Int
}

/// An item.
record Item {
  /// Its name.
  name: String
  /// Whether its count shows.
  shown: Bool
  /// How many.
  count: Int
  /// Its goal.
  goal: Goal
  /// What it aims at.
  target: Target(goal)
  /// Its parts.
  parts: [Part]

  check count > 0 else "count must be positive"
}

view Part {
  title "{label}"
}

view Item {
  title "Item {name}"
  subtitle "{goal}"
  count { when: shown }
  show "Summary" "{name} x{count}"
}

/// The items.
let items: table Item = {
  sword { name: "Sword", shown: false, count: 0, goal: kill, target: "wolf", parts: [{ label: "Blade", size: 2 }, { label: "Blade", size: 3 }] }
  bread { name: "Bread", shown: true, count: 1, goal: collect, target: 7, parts: [] }
}
`,
	"a/a.fr.canon": "package a\ntranslation fr\n\nItem.title \"Objet {name}\"\n",
	"b/b.canon": `/// B.
package b

import a

/// A box.
record Box {
  /// Its volume.
  volume: Int

  check volume > 0 else "empty box"
}

/// The box.
let box: Box = { volume: 0 }
`,
	"c/c.canon": "/// C.\npackage c\n\n/// N.\nlet n: Int = 1\n",
}

// evaluate is p's Evaluate of path in lang, failing the test on an error.
func evaluate(t *testing.T, p *canon.Project, path, lang string) *canon.EvalResult {
	t.Helper()
	res, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: path, Lang: lang})
	if err != nil {
		t.Fatalf("Evaluate(%s, %q): %v", path, lang, err)
	}
	return res
}

// API.md V4a: the JSON form of a result writes every key in declaration order, lowerCamel, an
// empty collection as [] or {}, map keys in byte order; a request omits base, draft and lang.
func TestEvaluateJSON(t *testing.T) {
	p, _ := openLaw(t, evalLaw)
	cases := []struct {
		name string
		res  any
	}{
		{"zero", canon.EvalResult{}},
		{"zero_heading", canon.Heading{}},
		{"sword_fr", evaluate(t, p, "a:items.sword", "fr")},
		{"items", evaluate(t, p, "items", "")},
		{"request", canon.EvalRequest{Path: "a:items"}},
	}
	for _, c := range cases {
		got, err := json.Marshal(c.res)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		golden(t, filepath.Join("testdata", "evaluate", c.name+".json"), append(got, '\n'))
	}
}

// golden compares got with the file name, or writes it under -update (DOCTRINE §4).
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%v (run go test ./api -run TestEvaluateJSON -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs:\n got %s\nwant %s", name, got, want)
	}
}

// API.md V8: texts are in the request's Lang, else Options.Lang; a language the project lacks
// falls back to the source everywhere, Fallback true.
func TestEvaluateLang(t *testing.T) {
	opts := project(evalLaw)
	opts.Lang = "fr"
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, c := range []struct {
		lang string
		want canon.Text
	}{
		{"", canon.Text{Value: "Objet Sword", OK: true}},
		{"en", canon.Text{Value: "Item Sword", OK: true}},
		{"fr", canon.Text{Value: "Objet Sword", OK: true}},
		{"de", canon.Text{Value: "Item Sword", OK: true, Fallback: true}},
	} {
		if got := evaluate(t, p, "a:items.sword", c.lang).Title; got != c.want {
			t.Errorf("API.md V8 Lang %q: title %+v, want %+v", c.lang, got, c.want)
		}
	}
}

// API.md V7, API.md V9, API.md P10: a canonical, qualified path; without a view a value is titled
// by its name, a field by its name, an entry, keyed element or map entry by its key, a plain
// element by `#<n>` from 1; alone or among its collection's headings, an element has one title.
func TestEvaluateTitlesWithoutView(t *testing.T) {
	p := openValueLaw(t)
	for _, c := range []struct{ path, canonical, title, owner, key string }{
		{"config", "a:config", "config", "", ""},
		{"config.server", "a:config.server", "server", "", ""},
		{"items[sword]", "a:items.sword", "sword", "a:items", "sword"},
		{"levels[#1]", "a:levels[7]", "7", "a:levels", "[7]"},
		{"nums[1]", "a:nums[1]", "#2", "a:nums", "[1]"},
		{"names[one]", "a:names[one]", "one", "a:names", "[one]"},
		{"names[#0]", `a:names["two words"]`, "two words", "a:names", `["two words"]`},
	} {
		res := evaluate(t, p, c.path, "")
		if res.Path != c.canonical || res.Title != (canon.Text{Value: c.title, OK: true}) {
			t.Errorf("API.md V7, API.md P10 %s: path %q title %+v, want %q %q", c.path, res.Path, res.Title, c.canonical, c.title)
		}
		if c.owner == "" {
			continue
		}
		if h, ok := evaluate(t, p, c.owner, "").Headings[c.key]; !ok || h.Title != res.Title {
			t.Errorf("API.md V6, API.md V9 %s: heading %q is %+v (%v), alone %+v", c.path, c.key, h.Title, ok, res.Title)
		}
	}
}

// API.md V10a, API.md E17: the findings at the path or below, with their reads; the summary
// covers the path's package and every package importing it.
func TestEvaluateFindings(t *testing.T) {
	p, _ := openLaw(t, evalLaw)
	for _, c := range []struct {
		path     string
		findings []string // path and reads of each finding
		summary  canon.Summary
	}{
		{"a:items", []string{"items.sword [count]"}, canon.Summary{Errors: 2, Packages: 2}},
		{"a:items.sword", []string{"items.sword [count]"}, canon.Summary{Errors: 2, Packages: 2}},
		{"a:items.sword.name", nil, canon.Summary{Errors: 2, Packages: 2}},
		{"a:items.bread", nil, canon.Summary{Errors: 2, Packages: 2}},
		{"b:box", []string{"box [volume]"}, canon.Summary{Errors: 1, Packages: 1}},
		{"c:n", nil, canon.Summary{Packages: 1}},
	} {
		res := evaluate(t, p, c.path, "")
		var got []string
		for _, f := range res.Findings {
			got = append(got, f.Path+" ["+strings.Join(f.Reads, " ")+"]")
		}
		if !slices.Equal(got, c.findings) || !summaryEqual(res.Summary, c.summary) {
			t.Errorf("API.md V10a %s: findings %q summary %+v, want %q %+v", c.path, got, res.Summary, c.findings, c.summary)
		}
	}
}

func summaryEqual(a, b canon.Summary) bool {
	return a.Errors == b.Errors && a.Warnings == b.Warnings && a.Packages == b.Packages && len(a.Truncated) == len(b.Truncated)
}

// API.md R5, API.md R6, API.md P7a, API.md S4, API.md S11, API.md O6, API.md V13: each refusal is
// its sentinel, a draft's by the rules of Edit.
func TestEvaluateErrors(t *testing.T) {
	p := openValueLaw(t)
	for _, c := range []struct {
		req  canon.EvalRequest
		want error
	}{
		{canon.EvalRequest{Path: "config..x"}, canon.ErrBadPath},
		{canon.EvalRequest{Path: "nowhere"}, canon.ErrNoPath},
		{canon.EvalRequest{Path: "limit"}, canon.ErrAmbiguousPath},
		{canon.EvalRequest{Path: "bad"}, canon.ErrNoValue},
		{canon.EvalRequest{Path: "gen.key"}, canon.ErrInputField},
		{canon.EvalRequest{Path: "a:Color.green"}, canon.ErrBadOp},
		{canon.EvalRequest{Path: "config", Base: "r1:00"}, canon.ErrStale},
		{canon.EvalRequest{Path: "config", Draft: []canon.Op{canon.Set("a:config.rate", canon.Str("x"))}}, canon.ErrBadValue},
		{canon.EvalRequest{Path: "config", Draft: []canon.Op{canon.Set("a:config..rate", canon.Int(1))}}, canon.ErrBadPath},
	} {
		if _, err := p.Evaluate(context.Background(), c.req); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v, want %v", c.req, err, c.want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Evaluate(ctx, canon.EvalRequest{Path: "config"}); !errors.Is(err, context.Canceled) {
		t.Errorf("API.md S11: a cancelled Evaluate: %v", err)
	}
	_ = p.Close()
	if _, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: "config"}); !errors.Is(err, canon.ErrClosed) {
		t.Errorf("API.md O6: Evaluate after Close: %v", err)
	}
}

// staleChange is one change after a base: files written, files removed, and what Evaluate of
// a:items against the base names stale (nil: not stale).
type staleChange struct {
	name    string
	write   map[string]string
	remove  []string
	stale   []string
	headsAt string // a heading title the current sources then give, "" to skip
}

// staleChanges are the probes of log-2026-09-29 M4 U5a-r and U5a-r2, each from a fresh base.
var staleChanges = []staleChange{
	{name: "an unrelated package's load", write: map[string]string{"d/d.json": "[2]\n"}},
	{name: "an importer's source", write: map[string]string{"b/b.canon": strings.Replace(evalLaw["b/b.canon"], "volume: 0", "volume: 1", 1)}, stale: []string{"b/b.canon"}},
	{name: "the path's source", write: map[string]string{"a/a.canon": strings.Replace(evalLaw["a/a.canon"], "Sword", "Axe", 1)}, stale: []string{"a/a.canon"}, headsAt: "Item Axe"},
	{name: "an unrelated source", write: map[string]string{"c/c.canon": evalLaw["c/c.canon"] + "\n/// M.\nlet m: Int = 2\n"}, stale: []string{"c/c.canon"}},
	{name: "an importer dropping its import", write: map[string]string{"b/b.canon": strings.Replace(evalLaw["b/b.canon"], "import a\n", "", 1)}, stale: []string{"b/b.canon"}},
	{name: "a new importing package", write: map[string]string{"e/e.canon": "/// E.\npackage e\n\nimport a\n\n/// N.\nlet n: Int = 1\n"}, stale: []string{"e/e.canon"}},
	{name: "a deleted package", remove: []string{"b/b.canon"}, stale: []string{"b"}},
	{name: "project.canon", write: map[string]string{"project.canon": strings.Replace(evalLaw["project.canon"], "[en, fr]", "[en, fr, de]", 1)}, stale: []string{"project.canon"}},
}

// API.md S5, API.md E17, API.md S6, API.md V13: an Evaluate is stale when the read set of the
// path's package or an importer changed, or any source or project.canon did, never for another
// package's load (log-2026-09-29 M4 U5a-r, U5a-r2); Evaluate never changes the revision.
func TestEvaluateStale(t *testing.T) {
	law := maps.Clone(evalLaw)
	law["d/d.canon"] = "/// D.\npackage d\n\n/// Xs.\nlet xs: [Int] = load(\"d.json\")\n"
	law["d/d.json"] = "[1]\n"
	for _, c := range staleChanges {
		p, opts := openLaw(t, law)
		fsys := opts.FS.(*memFS)
		base := evaluate(t, p, "a:items", "").Revision // d.json read: in the read set from now (S3)
		if evaluate(t, p, "a:items", "").Revision != base || p.Revision() != base {
			t.Errorf("API.md V13: Evaluate changed the revision")
		}
		for _, f := range slices.Sorted(maps.Keys(c.write)) {
			if err := fsys.WriteFile("/law/"+f, []byte(c.write[f])); err != nil {
				t.Fatal(err)
			}
		}
		for _, f := range c.remove {
			if err := fsys.Remove("/law/" + f); err != nil {
				t.Fatal(err)
			}
		}
		_, err := p.Evaluate(context.Background(), canon.EvalRequest{Base: base, Path: "a:items"})
		var se *canon.StaleError
		if c.stale == nil && err != nil || c.stale != nil && (!errors.As(err, &se) || !slices.Equal(se.Files, c.stale)) {
			t.Errorf("API.md S5 after %s: %v, want stale %q", c.name, err, c.stale)
		}
		res, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: "a:items"})
		if err != nil || c.headsAt != "" && res.Headings["sword"].Title.Value != c.headsAt {
			t.Errorf("API.md S6 after %s: without a base, the current sources: %v", c.name, err)
		}
	}
}

// API.md S7, API.md S8: Evaluate runs in parallel with Check, ViewModel and itself, each call's
// result the one it gives alone (run under -race).
func TestEvaluateConcurrentWithCheck(t *testing.T) {
	p, _ := openLaw(t, evalLaw)
	reqs := []canon.EvalRequest{{Path: "a:items.sword", Lang: "fr"}, {Path: "a:items"}, {Path: "b:box"}}
	want := make([][]byte, len(reqs))
	for i, r := range reqs {
		want[i] = evalJSON(t, p, r)
	}
	const rounds = 4
	var wg sync.WaitGroup
	for range rounds {
		for i, r := range reqs {
			wg.Go(func() {
				if got := evalJSON(t, p, r); !bytes.Equal(got, want[i]) {
					t.Errorf("a concurrent Evaluate(%s) differs: %s", r.Path, got)
				}
			})
		}
		wg.Go(func() {
			if _, err := p.Check(context.Background()); err != nil {
				t.Error(err)
			}
		})
		wg.Go(func() {
			if _, err := p.ViewModel(context.Background(), "a"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

// evalJSON is the JSON form of r's result on p.
func evalJSON(t *testing.T, p *canon.Project, r canon.EvalRequest) []byte {
	t.Helper()
	res, err := p.Evaluate(context.Background(), r)
	if err != nil {
		t.Error(err)
		return nil
	}
	got, err := json.Marshal(res)
	if err != nil {
		t.Error(err)
	}
	return got
}

// searchRow is one row of a view model's search index (VIEWMODEL.md 12.8).
type searchRow struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Retired  bool   `json:"retired"`
}

// searchColl is one collection of the search index.
type searchColl struct {
	KeyType string      `json:"keyType"`
	Rows    []searchRow `json:"rows"`
}

// searchIndex is the part of a view model the agreement test reads.
type searchIndex struct {
	Search map[string]searchColl `json:"search"`
}

// API.md V6, API.md V7: over the examples with views, each collection of the search index heads
// every row as the view model titles it (VIEWMODEL.md S9; its key without a title), retired alike.
func TestEvaluateAgreesWithViewModel(t *testing.T) {
	p, _ := openViewExamples(t)
	for _, pkg := range []string{"pipeline", "resource.farm", "resource.events", "resource.heistia", "resource.vocab"} {
		m, err := p.ViewModel(context.Background(), pkg)
		if err != nil {
			t.Fatal(err)
		}
		var idx searchIndex
		if err := json.Unmarshal(m.JSON(), &idx); err != nil {
			t.Fatal(err)
		}
		for _, id := range slices.Sorted(maps.Keys(idx.Search)) {
			agree(t, p, id, idx.Search[id])
		}
	}
}

// agree fails unless id's Evaluate heads exactly coll's rows, each as its row titles and
// subtitles it, and each element's own Evaluate gives its title before S9 disambiguates it
// (log-2026-09-29 M4 U5a-r: disambiguation is relative to a list).
func agree(t *testing.T, p *canon.Project, id string, coll searchColl) {
	t.Helper()
	res := evaluate(t, p, id, "")
	table := mustValue(t, p, id).Kind == canon.KindTable
	bases := map[string]int{}
	for _, row := range coll.Rows {
		bases[strings.TrimSuffix(row.Title, " ("+row.Key+")")]++
	}
	var wg sync.WaitGroup // concurrent Evaluates share one analysis (API.md S8)
	for _, row := range coll.Rows {
		rel := relKey(table, coll.KeyType == "int", row.Key)
		h, ok := res.Headings[rel]
		want := cmp.Or(row.Title, row.Key)
		if !ok || h.Title != (canon.Text{Value: want, OK: true}) || h.Subtitle.Value != row.Subtitle || h.Retired != row.Retired {
			t.Errorf("API.md V6 %s %s: heading %+v (%v), view model %+v", id, rel, h, ok, row)
		}
		if base := strings.TrimSuffix(want, " ("+row.Key+")"); base != want && bases[base] > 1 {
			want = base
		}
		path := id + rel
		if !strings.HasPrefix(rel, "[") {
			path = id + "." + rel
		}
		wg.Go(func() {
			own, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: path})
			if err != nil || own.Title != (canon.Text{Value: want, OK: true}) {
				t.Errorf("API.md V7 %s: own title %+v (%v), want %q", path, own, err, want)
			}
		})
	}
	wg.Wait()
	if len(res.Headings) != len(coll.Rows) {
		t.Errorf("API.md V6 %s: %d headings, %d rows", id, len(res.Headings), len(coll.Rows))
	}
}

// word is a word of SPEC 2.4.
var word = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// relKey is the canonical path of an element keyed key relative to its collection (API.md P8,
// API.md P9): a table entry `key` when key is a word, else `[key]`, a non-word string key written
// as WIRE.md writes strings.
func relKey(table, intKey bool, key string) string {
	isWord := !intKey && key != "_" && word.MatchString(key)
	switch {
	case table && isWord:
		return key
	case isWord || intKey:
		return "[" + key + "]"
	}
	return "[" + string(diag.AppendJSONString(nil, key)) + "]"
}

// controlLaw holds two fields of one collection type, one hinted `text` by its record's view.
var controlLaw = map[string]string{
	"project.canon": "project demo {\n  canon: \"0.1\"\n}\n",
	"a/a.canon": `/// A.
package a

/// Stats.
record Stats {
  /// Hit points.
  hp: Int
}

view Stats {
  title "Stats"
}

/// A part.
record Part {
  /// Its label.
  label: String
  /// Its stats.
  stats: Stats
}

view Part {
  title "{label}"
  columns { label, stats }
}

/// A box.
record Box {
  /// Shown as text.
  hinted: [Part]
  /// Shown as its type makes it.
  plain: [Part]
}

view Box {
  hinted { control: text }
}

/// The box.
let box: Box = { hinted: [{ label: "a", stats: { hp: 1 } }], plain: [{ label: "b", stats: { hp: 2 } }] }
`,
}

// API.md V6a (VIEWMODEL.md C1, T8): a collection held by a field takes the field's control, at
// its own path as in its record's form: a `text` hint has no cells where the type makes a table.
func TestEvaluateFieldControl(t *testing.T) {
	p, _ := openLaw(t, controlLaw)
	box := evaluate(t, p, "a:box", "")
	cells := map[string]canon.Text{"stats": {Value: "Stats", OK: true}}
	for _, c := range []struct {
		got  map[string]canon.Text
		want map[string]canon.Text
	}{
		{evaluate(t, p, "a:box.hinted", "").Headings["[0]"].Cells, map[string]canon.Text{}},
		{box.Headings["hinted[0]"].Cells, map[string]canon.Text{}},
		{evaluate(t, p, "a:box.plain", "").Headings["[0]"].Cells, cells},
		{box.Headings["plain[0]"].Cells, cells},
	} {
		if !maps.Equal(c.got, c.want) {
			t.Errorf("API.md V6a: cells %v, want %v", c.got, c.want)
		}
	}
}

// API.md V8, API.md V9 over examples/resource/farm in French: the farm's translated title, its
// models headed by `modelTypes[<typeId>]`.
func TestEvaluateFarmInFrench(t *testing.T) {
	p, _ := openViewExamples(t)
	res := evaluate(t, p, "resource.farm:farm", "fr")
	if res.Title != (canon.Text{Value: "Ferme", OK: true}) || res.Path != "resource.farm:farm" {
		t.Errorf("API.md V8: %s title %+v", res.Path, res.Title)
	}
	model, ok := res.Headings["modelTypes[1]"]
	if !ok || !model.Title.OK || model.Title.Value == "" {
		t.Errorf("API.md V9: modelTypes[1] heading %+v (%v) among %d", model, ok, len(res.Headings))
	}
}

// API.md V5, API.md V6 over examples/resource/heistia: every task of the form is headed, its
// title rendered; the value's own title is its view's.
func TestEvaluateHeistia(t *testing.T) {
	p, _ := openViewExamples(t)
	res := evaluate(t, p, "resource.heistia:heistia", "")
	tasks := mustValue(t, p, "resource.heistia:heistia.tasks")
	if res.Title != (canon.Text{Value: "Heistia", OK: true}) || tasks.Len() == 0 {
		t.Errorf("API.md V7: title %+v, %d tasks", res.Title, tasks.Len())
	}
	for i := range tasks.Len() {
		key := "tasks[" + strconv.Itoa(i) + "]"
		if h, ok := res.Headings[key]; !ok || !h.Title.OK || h.Title.Value == "" {
			t.Errorf("API.md V6: %s heading %+v (%v)", key, h, ok)
		}
	}
}
