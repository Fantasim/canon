package check_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	builtPackage = "a"
	builtFile    = "a/a.canon"
	testObject   = "test"
)

// syntaxCase is a program with a syntax error: the error codes it reports, the declarations broken.
type syntaxCase struct {
	name             string
	src              string
	want             []diag.Code
	broken, unbroken []string
}

// DECISIONS 214, TYPES.md §1, GRAMMAR.md §5.10, §6.1: misplaced constructs are checked; their declarations break.
var syntaxCases = []syntaxCase{
	{name: "§5.10 return in a test block, its operand still checked",
		src:  "test \"t\" { return 1 + \"a\" }\nlet n: Int = 1\n",
		want: []diag.Code{diag.E1135.Def().Code, diag.E3007.Def().Code}, broken: []string{testObject}, unbroken: []string{"n"}},
	{name: "§5.10 expect in a function, its operand still checked",
		src:  "fn f() -> Bool {\n  expect nope\n  return true\n}\nlet b: Bool = f()\n",
		want: []diag.Code{diag.E1130.Def().Code, diag.E2102.Def().Code}, broken: []string{"f", "b"}},
	{name: "§6.1 brace literal in a header, its items still checked",
		src: "record P { a: Int }\nfn g(x: P) -> Int {\n  if x == {a: nope} { return 1 }\n  return 0\n}\n" +
			"let n: Int = g(P { a: 1 })\n",
		want: []diag.Code{diag.E1129.Def().Code, diag.E2102.Def().Code}, broken: []string{"g", "n"}, unbroken: []string{"P"}},
	{name: "§5.10 break outside a loop in a function a let calls",
		src:  "fn h() -> Int {\n  break\n  return 1\n}\nlet y: Int = h()\nlet z: Int = 2\n",
		want: []diag.Code{diag.E1134.Def().Code}, broken: []string{"h", "y"}, unbroken: []string{"z"}},
	{name: "§5.10 break outside a loop, nothing else wrong",
		src:  "fn f() -> Int {\n  break\n  return 0\n}\n",
		want: []diag.Code{diag.E1134.Def().Code}, broken: []string{"f"}},
	{name: "§5.10 expect in a function, nothing else wrong",
		src:  "fn f() -> Int {\n  expect true\n  return 0\n}\n",
		want: []diag.Code{diag.E1130.Def().Code}, broken: []string{"f"}},
	{name: "§5.10 return in a test block, nothing else wrong",
		src:  "test \"t\" { return 1 }\n",
		want: []diag.Code{diag.E1135.Def().Code}, broken: []string{testObject}},
	{name: "§2.6 bad format spec in a record check breaks the record (DECISIONS 209)",
		src:  "record R {\n  n: Int\n  check n > 0 else \"bad {n:x}\"\n}\nlet rs: [R] = [R { n: 0 }]\nlet m: Int = 3\n",
		want: []diag.Code{diag.E1101.Def().Code}, broken: []string{"R", "rs"}, unbroken: []string{"m"}},
	{name: "§5.10 a syntax error in a method breaks only its callers (DECISIONS 209)",
		src: "record S {\n  n: Int\n  fn twice() -> Int {\n    break\n    return n * 2\n  }\n}\n" +
			"let ss: [S] = [S { n: 1 }]\nlet t2: Int = ss[0].twice()\n",
		want: []diag.Code{diag.E1134.Def().Code}, broken: []string{"twice", "t2"}, unbroken: []string{"S", "ss"}},
	{name: "TYPES.md §1 a static error in a method breaks a let calling it through a selector",
		src: "record S {\n  n: Int\n  fn twice() -> Int { return n * nope }\n}\n" +
			"let ss: [S] = [S { n: 1 }]\nlet t2: Int = ss[0].twice()\n",
		want: []diag.Code{diag.E2102.Def().Code}, broken: []string{"twice", "t2"}, unbroken: []string{"S", "ss"}},
}

func (sc syntaxCase) source() string {
	return "package " + builtPackage + "\n\n" + sc.src
}

// built is one checked file: its program, its tree, its rendered findings and its error codes.
type built struct {
	prog  *check.Program
	file  *syntax.File
	out   string
	codes []diag.Code
}

// checkBuilt checks one file of package a, its parse findings in the package's bag as a build
// puts them (project.Reader); codes are its errors' codes, sorted.
func checkBuilt(t *testing.T, text string) built {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add(builtFile, "/"+builtFile, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{builtPackage: diag.NewBag(set, builtPackage)}
	f := syntax.Parse(src, syntax.FileSource, bags[builtPackage])
	prog := check.Check(context.Background(), exampleProject(), []*syntax.File{f}, bags, literalFolder{})
	return built{prog: prog, file: f, out: render(t, set, diag.NewBag(set, ""), bags), codes: errorCodes(bags[builtPackage].Findings())}
}

// builtObjects are the objects of the checked file by name, a test under testObject.
func builtObjects(prog *check.Program, f *syntax.File) map[string]check.Object {
	out := shopObjects(prog, f)
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			if o.Kind() == check.ObjTest {
				out[testObject] = o
			}
		}
	}
	return out
}

func TestSyntaxErrorBreaksItsDeclaration(t *testing.T) {
	for _, sc := range syntaxCases {
		t.Run(sc.name, func(t *testing.T) {
			b := checkBuilt(t, sc.source())
			prog, f, out := b.prog, b.file, b.out
			if want := sortedCodes(sc.want); !slices.Equal(b.codes, want) {
				t.Errorf("codes %v, want %v:\n%s", b.codes, want, out)
			}
			objs := builtObjects(prog, f)
			for _, n := range sc.broken {
				if o := objs[n]; o == nil || !prog.Info.Broken[o] {
					t.Errorf("%s is not broken:\n%s", n, out)
				}
			}
			for _, n := range sc.unbroken {
				if o := objs[n]; o == nil || prog.Info.Broken[o] {
					t.Errorf("%s is broken, or missing:\n%s", n, out)
				}
			}
			for _, g := range gaps(f, prog.Info, brokenOrHolding(prog)) {
				t.Errorf("unbroken declaration with a gap: %s", g)
			}
		})
	}
}

// brokenOrHolding reports a declaration broken or holding a broken method, whose Info may have gaps (DECISIONS 209).
func brokenOrHolding(prog *check.Program) func(syntax.Decl) bool {
	broken := brokenDecl(prog)
	return func(d syntax.Decl) bool {
		holds := false
		syntax.Inspect(d, func(n syntax.Node) bool {
			if fn, ok := n.(*syntax.FnDecl); ok && n != d {
				o := prog.Info.Defs[fn.Name]
				holds = holds || o == nil || prog.Info.Broken[o]
			}
			return !holds
		})
		return holds || broken(d)
	}
}

// EVALUATION.md §1, DECISIONS 214: a build never evaluates what a syntax error broke.
func TestSyntaxErrorIsNeverEvaluated(t *testing.T) {
	for _, sc := range syntaxCases {
		t.Run(sc.name, func(t *testing.T) {
			res := buildOne(t, sc.source())
			if got, want := errorCodes(res.List), sortedCodes(sc.want); !slices.Equal(got, want) {
				t.Errorf("codes %v, want %v", got, want)
			}
		})
	}
}

// buildOne checks package a of a one-file project through a build (build.Open, then Check).
func buildOne(t *testing.T, text string) *build.Result {
	t.Helper()
	fsys := shopFS{
		shopDir + "/" + projectFile: {Data: []byte(shopProject)},
		shopDir + "/" + builtFile:   {Data: []byte(text)},
	}
	p, err := build.Open(fsys, "/"+shopDir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), []string{builtPackage})
	if errors.Is(err, build.ErrInternal) {
		t.Fatalf("the evaluator met a state the checker excludes: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// errorCodes are the codes of the errors among findings, sorted; warnings (W1002) left out.
func errorCodes(findings []diag.Finding) []diag.Code {
	var out []diag.Code
	for _, f := range findings {
		if f.Severity == diag.Error {
			out = append(out, f.Code)
		}
	}
	return sortedCodes(out)
}

func sortedCodes(codes []diag.Code) []diag.Code {
	out := slices.Clone(codes)
	slices.Sort(out)
	return out
}

// DECISIONS 214, API.md F7: a truncated bag still breaks every declaration holding a syntax error.
func TestSyntaxErrorTruncatedBag(t *testing.T) {
	text := "package a\n\nfn f() -> Int {\n  break\n  return 0\n}\n\nfn g() -> Int {\n  continue\n  return 0\n}\n"
	set := &source.FileSet{}
	src, err := set.Add(builtFile, "/"+builtFile, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(set, builtPackage)
	bag.Truncate(1)
	f := syntax.Parse(src, syntax.FileSource, bag)
	prog := check.Check(context.Background(), exampleProject(), []*syntax.File{f}, check.Bags{builtPackage: bag}, literalFolder{})
	objs := builtObjects(prog, f)
	for _, n := range []string{"f", "g"} {
		if o := objs[n]; o == nil || !prog.Info.Broken[o] {
			t.Errorf("%s is not broken", n)
		}
	}
}

// DECISIONS 214, EVALUATION.md §9.1: a syntax error in a layer file breaks the layer.
func TestSyntaxErrorBreaksItsLayer(t *testing.T) {
	files := map[string]string{
		builtFile:           "package a\n\n/// A config.\nlocal record Cfg {\n  /// Its name.\n  name: String\n}\n\nlocal let one: Cfg = { name: \"a\" }\n",
		"a/dev.layer.canon": "package a\nlayer dev\n\namend one {\n  name: \"b\\q\"\n}\n",
	}
	set := &source.FileSet{}
	bags := check.Bags{builtPackage: diag.NewBag(set, builtPackage)}
	var parsed []*syntax.File
	for _, name := range slices.Sorted(maps.Keys(files)) {
		src, err := set.Add(name, "/"+name, []byte(files[name]))
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, syntax.Parse(src, syntax.FileSource, bags[builtPackage]))
	}
	prog := check.Check(context.Background(), exampleProject(), parsed, bags, literalFolder{})
	layers := 0
	for _, f := range parsed {
		if f.Layer == nil {
			continue
		}
		layers++
		if o := prog.Info.Defs[f.Layer]; o == nil || !prog.Info.Broken[o] {
			t.Errorf("layer %s is not broken:\n%s", f.Layer.Name, render(t, set, diag.NewBag(set, ""), bags))
		}
	}
	if layers != 1 {
		t.Errorf("%d layer files, want 1", layers)
	}
}

// WIRE.md §2.1, DECISIONS 215: an out that does not resolve is the resolver's finding alone.
func TestUnresolvedOutHasNoPackageFinding(t *testing.T) {
	res := buildOne(t, "package a\n\nemit go { out: \"C:/gen/1b\" }\n")
	if got, want := errorCodes(res.List), []diag.Code{diag.E7001.Def().Code}; !slices.Equal(got, want) {
		t.Errorf("codes %v, want %v", got, want)
	}
}
