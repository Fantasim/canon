package check_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// shopProgram is package shop with one static error placed by a brokenCase: extra items of
// the record Item, of the case gold, and extra top-level declarations. Its data loads.
const shopProgram = `package shop

enum Rarity { common, rare, epic }

record Item {
  id: String
  rarity: Rarity
  price: Int(0..=1_000_000)
%s
}

variant Reward {
  gold {
    amount: Int
%s
  }
  nothing
}

let items: [Item] keyed by id = load.dir("data/*.json")
let rewards: [Reward] = [gold { amount: 3 }, nothing]
%s
`

const (
	shopDir     = "p"
	shopPackage = "shop"
	shopFile    = "shop/shop.canon"
	shopData    = "shop/data/IT_A.json"
	shopProject = "project demo { canon: \"0.1\" }\n"
	shopItem    = `{"id": "IT_A", "rarity": "epic", "price": 5}`
)

// brokenCase is one static error in one expression position.
type brokenCase struct {
	name              string
	record, gold, top string
	want              *diag.Def
}

// TYPES.md §1 step 3: an unknown name, member or enum member, or a mismatch, per expression position.
var brokenCases = []brokenCase{
	{name: "§5.1 bare member of the enum operand, record warn",
		record: `  warn cheap_epic: not (rarity == legendary and price < 1_000) else "an epic item this cheap"`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, record check", record: `  check price < LIMT else "too dear"`, want: diag.E2102.Def()},
	{name: "§3.5 unknown field, record check", record: `  check self.nope > 0 else "no nope"`, want: diag.E3003.Def()},
	{name: "§4.3 qualified unknown member, record check", record: `  check rarity != Rarity.legendary else "legendary"`, want: diag.E3003.Def()},
	{name: "§8.1 unknown enum built-in member, record check", record: `  check rarity.nope != "" else "nope"`, want: diag.E3003.Def()},
	{name: "§7.5 mismatch, record check", record: `  check rarity == 3 else "three"`, want: diag.E3002.Def()},
	{name: "§3.3 unknown name in a check message", record: `  check price < 10 else "too dear: {nope}"`, want: diag.E2102.Def()},
	{name: "VIEWMODEL G19 check at an unknown field", record: `  check price < 10 at nope else "at nope"`, want: diag.E1633.Def()},
	{name: "§3.3 unknown name, record block check",
		record: `  check { if rarity == legendary { fail(price, "legendary price") } }`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, case check", gold: `    check amount > LIMT else "too little"`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, method a record check calls",
		record: "  fn epic() -> Bool { return rarity == legendary }\n  check not epic() else \"epic\"", want: diag.E2102.Def()},
	{name: "§3.3 unknown name, field default", record: `  tier: Rarity = legendary`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, where predicate", record: `  bonus: Int where it != LIMT = 0`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, refinement bound", record: `  cap: Int(0..=LIMT) = 0`, want: diag.E2102.Def()},
	{name: "§5.1 bare member of the enum operand, let", top: `let epics = items.count(i => i.rarity == legendary)`, want: diag.E2102.Def()},
	{name: "§4.1 unknown member against an enum, let", top: `let best: Rarity = legendary`, want: diag.E2102.Def()},
	{name: "§5.2 unknown member in a record literal",
		top: `let one: Item = { id: "IT_X", rarity: legendary, price: 1 }`, want: diag.E2102.Def()},
	{name: "§5.1 unknown member in a list operand of in",
		top: `let rares = items.filter(i => i.rarity in [rare, legendary])`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, fn body a let calls",
		top: "fn isEpic(i: Item) -> Bool { return i.rarity == legendary }\nlet epicCount = items.count(i => isEpic(i))", want: diag.E2102.Def()},
	{name: "§3.3 unknown name, export fn",
		top: `export fn legendaries() -> Int { return items.count(i => i.rarity == legendary) }`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, package check", top: `check items.all(i => i.rarity != legendary) else "a legendary"`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, package block check",
		top: `check { for i in items { if i.rarity == legendary { fail(i, "legendary item") } } }`, want: diag.E2102.Def()},
	{name: "§12.6 unknown member as a match pattern",
		top: `let label = match items[0].rarity { legendary => "L", _ => "x" }`, want: diag.E3603.Def()},
	{name: "§3.3 unknown name, const", top: `const LIMIT = LIMT`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, test", top: `test "no legendary" { expect items.count(i => i.rarity == legendary) == 0 }`, want: diag.E2102.Def()},
	{name: "§3.3 unknown name, entry declaration", top: `entry items.IT_B { rarity: legendary, price: 1 }`, want: diag.E2102.Def()},
}

func (bc brokenCase) source() string {
	return fmt.Sprintf(shopProgram, bc.record, bc.gold, bc.top)
}

// checkShop checks package shop alone and renders its findings.
func checkShop(t *testing.T, text string) (*check.Program, *syntax.File, string) {
	t.Helper()
	set := &source.FileSet{}
	parse := diag.NewBag(set, "")
	src, err := set.Add(shopFile, "/"+shopFile, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, parse)
	bags := check.Bags{}
	prog := check.Check(context.Background(), exampleProject(), []*syntax.File{f}, bags, literalFolder{})
	return prog, f, render(t, set, parse, bags)
}

// TYPES.md §1, IMPLEMENTATION-PLAN §4.7: the error is reported; every unbroken declaration's Info is complete.
func TestStaticErrorBreaksItsDeclaration(t *testing.T) {
	for _, bc := range brokenCases {
		t.Run(bc.name, func(t *testing.T) {
			prog, f, out := checkShop(t, bc.source())
			if !strings.Contains(out, "["+string(bc.want.Code)+"]") {
				t.Fatalf("want %s, got:\n%s", bc.want.Code, out)
			}
			for _, g := range gaps(f, prog.Info, brokenDecl(prog)) {
				t.Errorf("unbroken declaration with a gap: %s", g)
			}
		})
	}
}

// DECISIONS 209: which of Item, items, Reward and rewards a member in error breaks.
func TestBrokenGranularity(t *testing.T) {
	for _, tc := range []struct {
		name             string
		record, gold     string
		broken, unbroken []string
	}{
		{name: "a broken record check breaks the record and what names it",
			record: `  check price < LIMT else "too dear"`, broken: []string{"Item", "items"}, unbroken: []string{"Reward", "rewards"}},
		{name: "a broken check naming its own record breaks it once",
			record: `  check self != Item { id: "IT_Z", rarity: legendary, price: 0 } else "the placeholder"`,
			broken: []string{"Item", "items"}, unbroken: []string{"Reward", "rewards"}},
		{name: "a sound check naming its own record breaks nothing",
			record:   `  check self != Item { id: "IT_Z", rarity: epic, price: 0 } else "the placeholder item"`,
			unbroken: []string{"Item", "items", "Reward", "rewards"}},
		{name: "a broken field default breaks the record",
			record: `  tier: Rarity = legendary`, broken: []string{"Item", "items"}, unbroken: []string{"Reward", "rewards"}},
		{name: "a broken case check breaks the variant",
			gold: `    check amount > LIMT else "too little"`, broken: []string{"Reward", "rewards"}, unbroken: []string{"Item", "items"}},
		{name: "a broken method no check calls breaks only itself",
			record: `  fn epic() -> Bool { return rarity == legendary }`, broken: []string{"epic"},
			unbroken: []string{"Item", "items", "Reward", "rewards"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prog, f, out := checkShop(t, fmt.Sprintf(shopProgram, tc.record, tc.gold, ""))
			objs := shopObjects(prog, f)
			for _, n := range tc.broken {
				if o := objs[n]; o == nil || !prog.Info.Broken[o] {
					t.Errorf("%s is not broken:\n%s", n, out)
				}
			}
			for _, n := range tc.unbroken {
				if o := objs[n]; o == nil || prog.Info.Broken[o] {
					t.Errorf("%s is broken, or missing:\n%s", n, out)
				}
			}
		})
	}
}

// shopObjects are the objects of package shop by name: its declarations and its methods.
func shopObjects(prog *check.Program, f *syntax.File) map[string]check.Object {
	out := map[string]check.Object{}
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			out[o.Name()] = o
		}
	}
	for _, fn := range nodesOf[*syntax.FnDecl](f) {
		if o := prog.Info.Defs[fn.Name]; o != nil && o.Kind() == check.ObjMethod {
			out[fn.Name.Name] = o
		}
	}
	return out
}

// brokenDecl reports a declaration whose object, as consumers find it (Package.Decls), is broken.
func brokenDecl(prog *check.Program) func(syntax.Decl) bool {
	objs := map[syntax.Node]check.Object{}
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			objs[o.Decl()] = o
		}
	}
	return func(d syntax.Decl) bool {
		o := objs[d]
		return o == nil || prog.Info.Broken[o]
	}
}

// EVALUATION.md §1: phases 1 to 7 report the static error and never evaluate what it broke.
func TestStaticErrorIsNeverEvaluated(t *testing.T) {
	for _, bc := range brokenCases {
		t.Run(bc.name, func(t *testing.T) {
			fsys := shopFS{
				shopDir + "/" + projectFile: {Data: []byte(shopProject)},
				shopDir + "/" + shopFile:    {Data: []byte(bc.source())},
				shopDir + "/" + shopData:    {Data: []byte(shopItem)},
			}
			p, err := build.Open(fsys, "/"+shopDir, build.Options{})
			if err != nil {
				t.Fatal(err)
			}
			res, err := p.Check(context.Background(), []string{shopPackage})
			if errors.Is(err, build.ErrInternal) {
				t.Fatalf("the evaluator met a state the checker excludes: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(res.List, func(f diag.Finding) bool { return f.Code == bc.want.Code }) {
				t.Errorf("want %s, got %v", bc.want.Code, codesOf(res.List))
			}
		})
	}
}

func codesOf(list []diag.Finding) []diag.Code {
	out := make([]diag.Code, 0, len(list))
	for _, f := range list {
		out = append(out, f.Code)
	}
	return out
}

// shopFS is a project in memory under /: fstest.MapFS with absolute names.
type shopFS fstest.MapFS

func relName(name string) string {
	if name == "/" {
		return "."
	}
	return strings.TrimPrefix(name, "/")
}

func (m shopFS) ReadFile(name string) ([]byte, error)  { return fstest.MapFS(m).ReadFile(relName(name)) }
func (m shopFS) Stat(name string) (fs.FileInfo, error) { return fstest.MapFS(m).Stat(relName(name)) }
func (m shopFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fstest.MapFS(m).ReadDir(relName(name))
}
