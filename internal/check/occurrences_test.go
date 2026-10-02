package check_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// occReaders is how many goroutines read the index at once.
const occReaders = 8

// occNames prints an occurrence kind as API.md E33 lists it.
var occNames = map[check.OccKind]string{
	check.OccDecl: "decl", check.OccUse: "use", check.OccQualified: "qualified", check.OccImport: "import",
	check.OccSelector: "selector", check.OccLiteralField: "literal-field", check.OccNamedArg: "named-arg",
	check.OccPattern: "pattern", check.OccKeyedBy: "keyed-by", check.OccRef: "ref", check.OccEntry: "entry",
	check.OccViewItem: "view-item", check.OccAmend: "amend", check.OccTranslationKey: "translation-key",
	check.OccEmitValues: "emit-values", check.OccFilesVar: "files-var",
}

// siteNames print a site as API.md R7 names its ref kind.
var siteNames = map[check.OccSite]string{
	check.SiteCode: "code", check.SiteView: "view", check.SiteCheck: "check", check.SiteLayer: "layer",
}

// occFixture is a program of two packages naming its declarations every way E33 lists: a view,
// a translation, a layer, an entry file, an emit and a `@files` template.
var occFixture = [][2]string{
	{"a/a.canon", `package a

record Item {
  name: String
  level: Int = 1
  kind: Kind
  check positive: level > 0 else "a level is positive"
}

enum Kind { sword, axe }

@files("items/{kind}/{id}.canon")
let items: table Item = {
  starter { name: "s", kind: sword }
}

record Rank {
  code: String
  title: String
}

let ranks: [Rank] keyed by code = [{ code: "p", title: "P" }]

fn grow(base: Int, extra: Int = 0) -> Int {
  return base + extra
}

let top: Int = grow(base: items.starter.level, extra: 1)

variant Prize {
  gold {
    amount: Int
    count: Int
  }
  gem {
    color: Kind
    count: Int
  }
}

record Holder {
  item: Item
  prize: Prize
}

let holder: Holder = { item: { name: "h", kind: axe }, prize: gold { amount: 1, count: 1 } }

@files("h/{item.kind}/{prize.amount}/{prize.count}/{id}.canon")
let holders: table Holder = {}

fn rank(k: Kind) -> Int {
  return match k {
    Kind.sword => 1
    axe => 2
  }
}

emit json { out: "out/", values: [items, holder] }
`},
	{"a/blade.canon", `package a

entry items.blade { name: "b", kind: axe }
`},
	{"a/a.view.canon", `package a

view Item {
  title "{name}"
  group main "Main" { name, level }
}
`},
	{"a/a.fr.canon", `package a
translation fr

Item.name "Nom"
Item.level "Niveau"
Rank.field.title "Titre"
`},
	{"a/dev.layer.canon", `package a
layer dev

amend holder {
  item.level: 3
  prize.amount: 2
  prize.count: 4
}
`},
	{"b/b.canon", `package b

import a { Item, items, grow }

record Offer {
  item: ref items
  level: Int
}

let offers: table Offer = {
  one { item: starter, level: 2 }
}

let first: Item = items.starter
let lv: Int = grow(base: first.level)
`},
	{"b/q.canon", `package b

import a

let other: a.Item = a.items.starter
let more: Int = a.grow(base: 1)
`},
}

// caseFixture reaches case fields through a variant, through a field typed by one case, and past
// a segment several cases declare, in a `@files` template and an amend path.
var caseFixture = [][2]string{
	{"p/p.canon", `package p

record Pos {
  x: Int
}

variant Prize {
  gold {
    amount: Int
    count: Int
    at: Pos
  }
  gem {
    count: Int
    at: Pos
  }
}

record Holder {
  prize: Prize
  pg: Prize.gold
}

@files("h/{prize.count}/{prize.amount}/{prize.at.x}/{pg.count}/{id}.canon")
let holders: table Holder = {}

let holder: Holder = { prize: gem { count: 1, at: { x: 1 } }, pg: gold { amount: 2, count: 2, at: { x: 2 } } }

record Bag {
  m: {String: Prize}
  l: [Prize]
}

let bag: Bag = { m: {}, l: [] }
`},
	{"p/dev.layer.canon", `package p
layer dev

amend holder {
  prize.count: 3
  pg.count: 4
  prize.at.x: 5
}

amend bag {
  m["k"].count: 6
  l[#0].amount: 7
}
`},
}

// checkSet checks files, each a path and its text, and renders every finding.
func checkSet(t *testing.T, files [][2]string) (*check.Program, []*syntax.File, string) {
	t.Helper()
	fs := &source.FileSet{}
	parse := diag.NewBag(fs, "")
	var parsed []*syntax.File
	for _, f := range files {
		src, err := fs.Add(f[0], "/"+f[0], []byte(f[1]))
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, syntax.Parse(src, syntax.FileSource, parse))
	}
	bags := check.Bags{}
	prog := check.Check(context.Background(), exampleProject(), parsed, bags, literalFolder{})
	return prog, parsed, render(t, fs, parse, bags)
}

// declAt names a declaration: the n-th declaring identifier name of file path, from 1.
type declAt struct {
	path, name string
	n          int
}

// declNamed is the object at declares.
func declNamed(t *testing.T, prog *check.Program, files []*syntax.File, at declAt) check.Object {
	t.Helper()
	i := slices.IndexFunc(files, func(f *syntax.File) bool { return f.Src.Path == at.path })
	if i < 0 {
		t.Fatalf("no file %s", at.path)
	}
	n := at.n
	for _, id := range nodesOf[*syntax.Ident](files[i]) {
		if o := prog.Info.Defs[id]; o != nil && id.Name == at.name {
			if n--; n == 0 {
				return o
			}
		}
	}
	t.Fatalf("no declaration of %s in %s", at.name, at.path)
	return nil
}

// occText prints each occurrence as `path:line:col kind site text [ambiguous]`.
func occText(occ []check.Occurrence) []string {
	out := make([]string, 0, len(occ))
	for _, o := range occ {
		line, col := o.File.Src.Position(o.Span.Start)
		text := string(o.File.Src.Content[o.Span.Start:o.Span.End])
		s := fmt.Sprintf("%s:%d:%d %s %s %s", o.File.Src.Path, line, col, occNames[o.Kind], siteNames[o.Site], text)
		if o.Ambiguous {
			s += " ambiguous"
		}
		out = append(out, s)
	}
	return out
}

// occCase is one object of the fixture, named by its n-th declaring identifier in path, and its
// occurrences as occText prints them.
type occCase struct {
	what string
	at   declAt
	want []string
}

// API.md E33, IMPLEMENTATION-PLAN §4.7: every recorded name, across files and packages.
func TestOccurrences(t *testing.T) {
	prog, files, out := checkSet(t, occFixture)
	if strings.Contains(out, "error[") {
		t.Fatalf("findings:\n%s", out)
	}
	gold := declNamed(t, prog, files, declAt{"a/a.canon", "count", 1})
	for _, f := range files {
		for _, seg := range nodesOf[*syntax.AmendSegment](f) {
			if o := prog.Info.NameUses[seg.Name]; seg.Name.Name == "count" && o != gold {
				t.Errorf("amend segment count: NameUses = %v, want the first case's field", o)
			}
		}
		for _, sel := range nodesOf[*syntax.SelectorExpr](f) {
			if o := prog.Info.NameUses[sel.Name]; sel.Name.Name == "count" && o != nil {
				t.Errorf("{prize.count}: NameUses = %v, want none (no one declaration)", o)
			}
		}
	}
	wantOccurrences(t, prog, files, occCases)
}

// wantOccurrences fails for each case whose object's occurrences are not its want.
func wantOccurrences(t *testing.T, prog *check.Program, files []*syntax.File, cases []occCase) {
	t.Helper()
	for _, tc := range cases {
		o := declNamed(t, prog, files, tc.at)
		if got := occText(prog.Occurrences(o)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: Occurrences(%s) =\n  %s\nwant\n  %s", tc.what, tc.at.name, strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "))
		}
	}
}

// IMPLEMENTATION-PLAN §4.7: one case's type is never ambiguous; past an ambiguous segment, one declaration is plain.
func TestOccurrencesThroughCases(t *testing.T) {
	prog, files, out := checkSet(t, caseFixture)
	if strings.Contains(out, "error[") {
		t.Fatalf("findings:\n%s", out)
	}
	tpl := nodesOf[*syntax.SelectorExpr](files[0])
	x := tpl[slices.IndexFunc(tpl, func(s *syntax.SelectorExpr) bool { return s.Name.Name == "x" })]
	if o := prog.Info.NameUses[x.Name]; o != declNamed(t, prog, files, declAt{"p/p.canon", "x", 1}) {
		t.Errorf("{prize.at.x}: NameUses = %v, want Pos.x", o)
	}
	wantOccurrences(t, prog, files, caseCases)
}

var caseCases = []occCase{
	{"gold.count: ambiguous through the variant and `m[\"k\"]`, plain through `pg: Prize.gold`", declAt{"p/p.canon", "count", 1}, []string{
		"p/dev.layer.canon:5:9 amend layer count ambiguous", "p/dev.layer.canon:6:6 amend layer count",
		"p/dev.layer.canon:11:10 amend layer count ambiguous", "p/p.canon:10:5 decl code count", "p/p.canon:24:18 files-var code count ambiguous",
		"p/p.canon:24:57 files-var code count", "p/p.canon:27:85 literal-field code count",
	}},
	{"gem.count: ambiguous through the variant only", declAt{"p/p.canon", "count", 2}, []string{
		"p/dev.layer.canon:5:9 amend layer count ambiguous", "p/dev.layer.canon:11:10 amend layer count ambiguous",
		"p/p.canon:14:5 decl code count",
		"p/p.canon:24:18 files-var code count ambiguous", "p/p.canon:27:37 literal-field code count",
	}},
	{"Pos.x: one declaration past the ambiguous `at`", declAt{"p/p.canon", "x", 1}, []string{
		"p/dev.layer.canon:7:12 amend layer x", "p/p.canon:4:3 decl code x", "p/p.canon:24:50 files-var code x",
		"p/p.canon:27:53 literal-field code x", "p/p.canon:27:101 literal-field code x",
	}},
	{"gold.at: the ambiguous segment", declAt{"p/p.canon", "at", 1}, []string{
		"p/dev.layer.canon:7:9 amend layer at ambiguous", "p/p.canon:11:5 decl code at",
		"p/p.canon:24:47 files-var code at ambiguous", "p/p.canon:27:95 literal-field code at",
	}},
	{"gold.amount: plain past `l[#0]`, one case declaring it", declAt{"p/p.canon", "amount", 1}, []string{
		"p/dev.layer.canon:12:9 amend layer amount", "p/p.canon:9:5 decl code amount",
		"p/p.canon:24:32 files-var code amount", "p/p.canon:27:74 literal-field code amount",
	}},
	{"pg, a field typed by one case", declAt{"p/p.canon", "pg", 1}, []string{
		"p/dev.layer.canon:6:3 amend layer pg", "p/p.canon:21:3 decl code pg", "p/p.canon:24:54 files-var code pg",
		"p/p.canon:27:63 literal-field code pg",
	}},
}

var occCases = []occCase{
	{"a type: uses, import list, qualified form, view target, translation keys", declAt{"a/a.canon", "Item", 1}, []string{
		"a/a.canon:3:8 decl code Item", "a/a.canon:13:18 use code Item", "a/a.canon:42:9 use code Item",
		"a/a.fr.canon:4:1 translation-key view Item", "a/a.fr.canon:5:1 translation-key view Item",
		"a/a.view.canon:3:6 view-item view Item", "b/b.canon:3:12 import code Item", "b/b.canon:14:12 use code Item",
		"b/q.canon:5:14 qualified code Item",
	}},
	{"a field used in two packages, a check, a view item, a translation key and an amend path", declAt{"a/a.canon", "level", 1}, []string{
		"a/a.canon:5:3 decl code level", "a/a.canon:7:19 use check level", "a/a.canon:28:41 selector code level",
		"a/a.fr.canon:5:6 translation-key view level", "a/a.view.canon:5:29 view-item view level",
		"a/dev.layer.canon:5:8 amend layer level", "b/b.canon:15:32 selector code level",
	}},
	{"a field: literal fields in a let and an entry file, @files variables", declAt{"a/a.canon", "kind", 1}, []string{
		"a/a.canon:6:3 decl code kind", "a/a.canon:12:16 files-var code kind", "a/a.canon:14:24 literal-field code kind",
		"a/a.canon:46:43 literal-field code kind", "a/a.canon:48:17 files-var code kind",
		"a/blade.canon:3:32 literal-field code kind",
	}},
	{"a table let: entry line, ref, emit values, import list, qualified form", declAt{"a/a.canon", "items", 1}, []string{
		"a/a.canon:13:5 decl code items", "a/a.canon:28:27 use code items", "a/a.canon:58:35 emit-values code items",
		"a/blade.canon:3:7 entry code items", "b/b.canon:3:18 import code items", "b/b.canon:6:13 ref code items",
		"b/b.canon:14:19 use code items", "b/q.canon:5:23 qualified code items",
	}},
	{"a keyed-by field", declAt{"a/a.canon", "code", 1}, []string{
		"a/a.canon:18:3 decl code code", "a/a.canon:22:28 keyed-by code code", "a/a.canon:22:38 literal-field code code",
	}},
	{"a function: uses, import list, qualified call", declAt{"a/a.canon", "grow", 1}, []string{
		"a/a.canon:24:4 decl code grow", "a/a.canon:28:16 use code grow", "b/b.canon:3:25 import code grow",
		"b/b.canon:15:15 use code grow", "b/q.canon:6:19 qualified code grow",
	}},
	{"a parameter: body use and named arguments in two packages", declAt{"a/a.canon", "base", 1}, []string{
		"a/a.canon:24:9 decl code base", "a/a.canon:25:10 use code base", "a/a.canon:28:21 named-arg code base",
		"b/b.canon:15:20 named-arg code base", "b/q.canon:6:24 named-arg code base",
	}},
	{"an enum member: a value and a qualified pattern", declAt{"a/a.canon", "sword", 1}, []string{
		"a/a.canon:10:13 decl code sword", "a/a.canon:14:30 use code sword", "a/a.canon:53:10 pattern code sword",
	}},
	{"an enum: a pattern's qualifier", declAt{"a/a.canon", "Kind", 1}, []string{
		"a/a.canon:6:9 use code Kind", "a/a.canon:10:6 decl code Kind", "a/a.canon:36:12 use code Kind",
		"a/a.canon:51:12 use code Kind", "a/a.canon:53:5 pattern code Kind",
	}},
	{"a let: emit values and an amend target", declAt{"a/a.canon", "holder", 1}, []string{
		"a/a.canon:46:5 decl code holder", "a/a.canon:58:42 emit-values code holder", "a/dev.layer.canon:4:7 amend layer holder",
	}},
	{"a field heading a @files path and an amend path", declAt{"a/a.canon", "item", 1}, []string{
		"a/a.canon:42:3 decl code item", "a/a.canon:46:24 literal-field code item", "a/a.canon:48:12 files-var code item",
		"a/dev.layer.canon:5:3 amend layer item",
	}},
	{"a field named by a template and read in a view", declAt{"a/a.canon", "name", 1}, []string{
		"a/a.canon:4:3 decl code name", "a/a.canon:14:13 literal-field code name", "a/a.canon:46:32 literal-field code name",
		"a/a.fr.canon:4:6 translation-key view name", "a/a.view.canon:4:11 use view name",
		"a/a.view.canon:5:23 view-item view name", "a/blade.canon:3:21 literal-field code name",
	}},
	{"an entry: a ref value in another package and selectors", declAt{"a/a.canon", "starter", 1}, []string{
		"a/a.canon:14:3 decl code starter", "a/a.canon:28:33 selector code starter", "b/b.canon:11:15 use code starter",
		"b/b.canon:14:25 selector code starter",
	}},
	{"a field whose name is a reserved segment: its key segment after the kind word (I18N.md K4)", declAt{"a/a.canon", "title", 1}, []string{
		"a/a.canon:19:3 decl code title", "a/a.canon:22:49 literal-field code title", "a/a.fr.canon:6:12 translation-key view title",
	}},
	{"a field of one case, through a variant in a @files path and an amend path", declAt{"a/a.canon", "amount", 1}, []string{
		"a/a.canon:32:5 decl code amount", "a/a.canon:46:70 literal-field code amount", "a/a.canon:48:30 files-var code amount",
		"a/dev.layer.canon:6:9 amend layer amount",
	}},
	{"a field two cases declare: `{prize.count}` and `prize.count` are each case's, ambiguous", declAt{"a/a.canon", "count", 1}, []string{
		"a/a.canon:33:5 decl code count", "a/a.canon:46:81 literal-field code count",
		"a/a.canon:48:45 files-var code count ambiguous", "a/dev.layer.canon:7:9 amend layer count ambiguous",
	}},
	{"the other case's field of that name", declAt{"a/a.canon", "count", 2}, []string{
		"a/a.canon:37:5 decl code count", "a/a.canon:48:45 files-var code count ambiguous",
		"a/dev.layer.canon:7:9 amend layer count ambiguous",
	}},
	{"a variant field heading template and amend paths", declAt{"a/a.canon", "prize", 1}, []string{
		"a/a.canon:43:3 decl code prize", "a/a.canon:46:56 literal-field code prize", "a/a.canon:48:24 files-var code prize",
		"a/a.canon:48:39 files-var code prize", "a/dev.layer.canon:6:3 amend layer prize", "a/dev.layer.canon:7:3 amend layer prize",
	}},
}

// API.md E33, IMPLEMENTATION-PLAN §4.7: a broken declaration lists only the names it resolved.
func TestOccurrencesOfBrokenDeclarations(t *testing.T) {
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", `package a

local let x: Int = 1

local let bad: Nope = x

local fn f() -> Int {
  return x + missing
}
`}})
	if want := "[" + string(diag.E2102.Def().Code) + "]"; !strings.Contains(out, want) {
		t.Fatalf("want %s, got:\n%s", want, out)
	}
	for _, tc := range []occCase{
		{"a let read by broken declarations", declAt{"a/a.canon", "x", 1}, []string{
			"a/a.canon:3:11 decl code x", "a/a.canon:5:23 use code x", "a/a.canon:8:10 use code x",
		}},
		{"a broken let", declAt{"a/a.canon", "bad", 1}, []string{"a/a.canon:5:11 decl code bad"}},
		{"a broken function", declAt{"a/a.canon", "f", 1}, []string{"a/a.canon:7:10 decl code f"}},
	} {
		o := declNamed(t, prog, files, tc.at)
		if got := occText(prog.Occurrences(o)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: Occurrences(%s) = %q, want %q", tc.what, tc.at.name, got, tc.want)
		}
		if tc.at.name != "x" && !prog.Info.Broken[o] {
			t.Errorf("%s: want broken", tc.at.name)
		}
	}
}

// IMPLEMENTATION-PLAN §4.7: a re-checked program indexes its own files; the old one keeps its.
func TestOccurrencesAfterRecheck(t *testing.T) {
	fs := &source.FileSet{}
	parse := func(path, text string) *syntax.File {
		src, err := fs.Add(path, "/"+path, []byte(text))
		if err != nil {
			t.Fatal(err)
		}
		return syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))
	}
	item := parse("shop/item.canon", "package shop\n\nlocal record Item {\n  price: Int\n}\n\nemit json { out: \"out/\", values: [items] }\n")
	letText := "package shop\n\n@files(\"{price}/{id}.canon\")\nlocal let items: table Item = { pear { price: %d } }\n"
	shop := parse("shop/shop.canon", fmt.Sprintf(letText, 1))
	old, s := check.CheckSession(context.Background(), exampleProject(), []*syntax.File{item, shop}, check.Bags{}, literalFolder{})
	edited := parse("shop/shop.canon", fmt.Sprintf(letText, 2))
	prog, _, ok := s.Recheck(context.Background(), []*syntax.File{edited}, check.Bags{}, literalFolder{})
	if !ok {
		t.Fatal("Recheck refused the edit")
	}
	want := []string{
		"shop/item.canon:4:3 decl code price", "shop/shop.canon:3:10 files-var code price",
		"shop/shop.canon:4:40 literal-field code price",
	}
	for _, p := range []*check.Program{old, prog} {
		price := declNamed(t, p, p.Packages[0].Files, declAt{"shop/item.canon", "price", 1})
		occ := p.Occurrences(price)
		if got := occText(occ); !slices.Equal(got, want) {
			t.Errorf("Occurrences(price) = %q, want %q", got, want)
		}
		if len(occ) == len(want) && occ[1].File != p.Packages[0].Files[1] {
			t.Errorf("the template is not in the program's own shop.canon")
		}
	}
	if old.Packages[0].Files[1] == prog.Packages[0].Files[1] {
		t.Error("the edit did not replace shop.canon")
	}
}

// IMPLEMENTATION-PLAN §4.7: one index, read concurrently; none for nil or a Program without Info.
func TestOccurrencesConcurrentAndEmpty(t *testing.T) {
	prog, files, _ := checkSet(t, occFixture)
	items := declNamed(t, prog, files, declAt{"a/a.canon", "items", 1})
	var wg sync.WaitGroup
	counts := make([]int, occReaders)
	for i := range counts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counts[i] = len(prog.Occurrences(items))
		}()
	}
	wg.Wait()
	if slices.Min(counts) != slices.Max(counts) || counts[0] == 0 {
		t.Errorf("concurrent counts differ or are empty: %v", counts)
	}
	got := prog.Occurrences(items)
	got[0].Kind = check.OccFilesVar
	if prog.Occurrences(items)[0].Kind != check.OccDecl {
		t.Error("a caller's change reached the index")
	}
	if prog.Occurrences(nil) != nil || (&check.Program{}).Occurrences(items) != nil {
		t.Error("want no occurrence for a nil object or a program without Info")
	}
}

// API.md E33, IMPLEMENTATION-PLAN §4.7: every Def, NameUses and Uses name of the examples is listed.
func TestOccurrencesCoverEveryExampleName(t *testing.T) {
	l := loadExamples(t)
	prog, _ := l.run(t)
	info := prog.Info
	seen := map[check.Object]map[source.Span]bool{}
	listed := func(o check.Object, f *syntax.File, n syntax.Node) bool {
		if seen[o] == nil {
			seen[o] = map[source.Span]bool{}
			for _, occ := range prog.Occurrences(o) {
				seen[o][occ.Span] = true
			}
		}
		return seen[o][f.Span(n)]
	}
	var defs, nameUses, uses int
	for _, f := range l.files {
		for _, id := range nodesOf[*syntax.Ident](f) {
			defs += countListed(t, f, id, info.Defs[id], listed)
			nameUses += countListed(t, f, id, info.NameUses[id], listed)
		}
		for _, x := range nodesOf[*syntax.IdentExpr](f) {
			uses += countListed(t, f, x, info.Uses[x], listed)
		}
	}
	if defs != len(info.Defs) || nameUses != len(info.NameUses) || uses != len(info.Uses) {
		t.Errorf("names found in the files: %d Defs, %d NameUses, %d Uses; Info holds %d, %d, %d",
			defs, nameUses, uses, len(info.Defs), len(info.NameUses), len(info.Uses))
	}
}

// countListed is 1 for a name naming o, failing when o's occurrences miss it; 0 for no object.
func countListed(t *testing.T, f *syntax.File, n syntax.Node, o check.Object, listed func(check.Object, *syntax.File, syntax.Node) bool) int {
	t.Helper()
	if o == nil {
		return 0
	}
	if !listed(o, f, n) {
		line, col := f.Src.Position(f.Span(n).Start)
		t.Errorf("%s:%d:%d %s: not among the occurrences of %s %s", f.Src.Path, line, col, n.Kind(), o.Kind(), o.Name())
	}
	return 1
}
