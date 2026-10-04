package canon_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// API.md O1: FindProject looks in dir and then in each parent, and returns the directory that
// holds project.canon.
func TestFindProjectParents(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "game", "items", "deeper")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "project.canon"), []byte("project a {\n  canon: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, filepath.Join(root, "game"), deep} {
		got, err := canon.FindProject(dir)
		if err != nil {
			t.Fatalf("FindProject(%s): %v", dir, err)
		}
		resolved, err := filepath.EvalSymlinks(got)
		if err != nil || resolved != root {
			t.Errorf("FindProject(%s) = %s (%v), want %s", dir, got, err, root)
		}
	}
}

// checkLaw is one package whose checks fail, named and unnamed, and one division by zero.
const checkLaw = `package teamboard

record Column {
  label: String(1..)
  statuses: [String]

  check not statuses.isEmpty() else "a column holds at least one status"
  check short: label.len() < 12 at label else "the label is too long"
}

let columns: stable table Column = {
  unclaimed { label: "Unclaimed", statuses: [] }
  archive { label: "Everything archived", statuses: ["done"] }
}

let zero: Int = 1 / 0
`

func checkFindings(t *testing.T) []canon.Finding {
	t.Helper()
	fs, _ := checkProject(t)
	return fs
}

func checkProject(t *testing.T) ([]canon.Finding, *canon.Project) {
	t.Helper()
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon":            "project a {\n  canon: \"0.1\"\n}\n",
		"teamboard/taxonomy.canon": checkLaw,
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Check(context.Background(), "teamboard")
	if err != nil {
		t.Fatal(err)
	}
	return res.Findings, p
}

func findingWith(t *testing.T, fs []canon.Finding, code string, line int) canon.Finding {
	t.Helper()
	for _, f := range fs {
		if f.Code == code && f.Line == line {
			return f
		}
	}
	t.Fatalf("no %s finding at line %d in %+v", code, line, fs)
	return canon.Finding{}
}

// API.md F1: Path is the canonical path without the package qualifier, so Package + ":" + Path
// resolves with Value, which returns the value the finding is about.
func TestFindingPathResolves(t *testing.T) {
	findings, p := checkProject(t)
	want := map[string]string{
		"columns.unclaimed":     `Column{label: "Unclaimed", statuses: []}`,
		"columns.archive.label": "Everything archived",
	}
	seen := 0
	for _, f := range findings {
		if f.Path == "" {
			if diag.Code(f.Code) != diag.W1002.Def().Code { // a missing doc comment is about no value
				t.Errorf("%s at line %d has no Path", f.Code, f.Line)
			}
			continue
		}
		v, err := p.Value(context.Background(), f.Package+":"+f.Path)
		if errors.Is(err, canon.ErrNoValue) { // a finding inside a value that failed: it still resolves
			continue
		}
		text, ok := want[f.Path]
		if !ok {
			t.Errorf("unexpected Path %q", f.Path)
			continue
		}
		seen++
		if err != nil || v.String() != text {
			t.Errorf("Value(%s:%s) = %v, %v; want %s", f.Package, f.Path, v, err, text)
		}
	}
	if seen != len(want) {
		t.Errorf("%d findings with a wanted path, want %d", seen, len(want))
	}
}

// API.md F3: a finding's code is one of the registry generated from ERRORS.md, the user checks
// are E5001, and the message of a code without arguments is its template.
func TestFindingCodesFromRegistry(t *testing.T) {
	findings := checkFindings(t)
	known := map[string]bool{}
	for _, d := range diag.Registry {
		known[string(d.Code)] = true
	}
	for _, f := range findings {
		if !known[f.Code] {
			t.Errorf("code %s is not in the registry", f.Code)
		}
	}
	div := findingWith(t, findings, string(diag.E4102.Def().Code), 16)
	if want := diag.E4102.Def().Variants[0].Template; div.Message != want {
		t.Errorf("message %q, want the template %q", div.Message, want)
	}
	user := findingWith(t, findings, string(diag.E5001.Def().Code), 12)
	if user.Message != "a column holds at least one status" {
		t.Errorf("user check message %q", user.Message)
	}
}

// API.md F4: a one-line check's span is CHK-02's, and Related holds the check with note `check <name>` or `check`.
func TestOneLineCheckFinding(t *testing.T) {
	findings := checkFindings(t)
	unnamed := findingWith(t, findings, string(diag.E5001.Def().Code), 12)
	if len(unnamed.Related) != 1 || unnamed.Related[0].Note != "check" || unnamed.Related[0].Line != 7 ||
		unnamed.Related[0].File != "teamboard/taxonomy.canon" || unnamed.Col != 3 {
		t.Errorf("unnamed check: %+v", unnamed)
	}
	named := findingWith(t, findings, string(diag.E5001.Def().Code), 13)
	if len(named.Related) != 1 || named.Related[0].Note != "check short" || named.Related[0].Line != 8 || named.Col != 20 {
		t.Errorf("named check: %+v", named)
	}
	if unnamed.Check != "" || named.Check != "short" {
		t.Errorf("Check names %q and %q", unnamed.Check, named.Check)
	}
}

// API.md F10, F11: the message line is two spaces, `<path>: ` when the path is not empty, then
// the message, each further line indented two spaces too; a layer adds `  set by layer <layer>`.
func TestFindingTextMessageAndLayer(t *testing.T) {
	codeE, codeW := string(diag.E3501.Def().Code), string(diag.W5001.Def().Code)
	span := canon.Span{File: "a/a.canon", Line: 3, Col: 4, EndLine: 3, EndCol: 5}
	findings := []canon.Finding{
		{Severity: canon.SeverityError, Code: codeE, Span: span, Package: "a", Path: "a.x[1]", Message: "first\nsecond\nthird", Layer: "gm"},
		{Severity: canon.SeverityWarning, Code: codeW, Package: "a", Message: "no path\nsecond"},
	}
	var buf bytes.Buffer
	sum := canon.Summary{Errors: 1, Warnings: 1, Packages: 1}
	if err := canon.WriteFindings(&buf, findings, canon.WriteOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	want := "warning[" + codeW + "]\n" +
		"  no path\n  second\n" +
		"\n" +
		"error[" + codeE + "]  a/a.canon:3:4\n" +
		"  a.x[1]: first\n  second\n  third\n" +
		"  set by layer gm\n" +
		"\n" +
		"1 error, 1 warning in 1 package (…)\n"
	if got := buf.String(); got != want {
		t.Errorf("text form:\n%s\nwant:\n%s", got, want)
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("line %q ends with a space", line)
		}
	}
}

// testsFirst is package b of the B3 project: passing and failing tests, in one file.
const testsFirst = `package b

fn div(n: Int) -> Int { return n / (n - n) }

test "b passes" {
  let x: Int = 1
  expect x + 1 == 2
}

test "b fails" {
  let x: Int = 1
  let xs: [Int] = [1, 2]
  expect x + 1 == 3
  expect x + 3 == 4
  expect div(x) fails "unrelated text"
  expect xs == [1,2,  3]
  expect x + 4 == 1_000
}
`

const testsSecond = `package a

test "a second file" {
  expect [1, 2].len() == 2
}
`

// testsX is package x, whose only file sorts after the file of its child package x.y.
const testsX = "package x\n\ntest \"x late file\" { expect true }\n"

// testsXY is package x.y, in x/y/y.canon: file-major order puts it before x/z.canon.
const testsXY = "package x.y\n\ntest \"x.y file\" { expect true }\n"

// API.md B3: Tests are in (package, file, line) order, package-major (x before x.y although
// x/y/y.canon sorts before x/z.canon); Expect is the source text of the expect statement, Expected
// and Got are canonical text forms (`[1,2,  3]` is `[1, 2, 3]`, `1_000` is `1000`).
func TestTestResultOrder(t *testing.T) {
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon": "project a {\n  canon: \"0.1\"\n}\n",
		"b/b.canon":     testsFirst,
		"a/z.canon":     testsSecond,
		"x/z.canon":     testsX,
		"x/y/y.canon":   testsXY,
		"a/a.canon":     "package a\n\ntest \"a first file, line 3\" { expect true }\n\ntest \"a first file, line 5\" { expect true }\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Test(context.Background(), canon.TestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range res.Tests {
		names = append(names, c.Package+"/"+c.Name)
	}
	want := []string{"a/a first file, line 3", "a/a first file, line 5", "a/a second file", "b/b passes", "b/b fails",
		"x/x late file", "x.y/x.y file"}
	if !slices.Equal(names, want) || res.Passed != 6 || res.Failed != 1 {
		t.Fatalf("order %q, passed %d failed %d: %+v", names, res.Passed, res.Failed, res.Tests)
	}
	failed := res.Tests[4]
	if failed.Passed || len(failed.Failures) != 4 {
		t.Fatalf("b fails: %+v", failed)
	}
	f, g, list, num := failed.Failures[0], failed.Failures[1], failed.Failures[2], failed.Failures[3]
	if f.Expect != "expect x + 1 == 3" || f.Expected != "3" || f.Got != "2" || f.Line != 13 {
		t.Errorf("failure: %+v", f)
	}
	if g.Expect != "expect div(x) fails \"unrelated text\"" || len(g.Findings) != 1 || g.Findings[0].Code != string(diag.E4102.Def().Code) {
		t.Errorf("fails failure: %+v", g)
	}
	if list.Expect != "expect xs == [1,2,  3]" || list.Expected != "[1, 2, 3]" || list.Got != "[1, 2]" {
		t.Errorf("list failure: %+v", list)
	}
	if num.Expect != "expect x + 4 == 1_000" || num.Expected != "1000" || num.Got != "5" {
		t.Errorf("number failure: %+v", num)
	}
}

// importerPackage is package a, which imports package b, with a stable table and a json emit.
const importerPackage = `/// A.
package a

import b

/// Tier.
record Tier {
  /// Weight.
  weight: Int = 1
}

/// Tiers.
let tiers: stable table Tier = { low {} }

/// Uses b.
let copy: Int = b.limit

emit json { out: "@out/a/" }
`

const brokenImport = `/// B.
package b

/// Limit.
let limit: Int = "not an int"
`

// API.md B1a: an error in an imported package, not selected, blocks code, data and the lock, and
// its error findings are reported with the selection's.
func TestBuildImportedErrorBlocks(t *testing.T) {
	fsys := newMemFS(map[string][]byte{
		"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": []byte(importerPackage),
		"/law/b/b.canon": []byte(brokenImport),
	})
	p := openTierProject(t, fsys)
	res, err := p.Build(context.Background(), canon.BuildOptions{Packages: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Check.HasErrors() || !slices.ContainsFunc(res.Check.Findings, func(f canon.Finding) bool { return f.Package == "b" && f.Severity == canon.SeverityError }) {
		t.Errorf("the imported package's error is not reported: %+v", res.Check.Findings)
	}
	for _, name := range []string{"/law/out/a/tiers.json", "/law/a/canon.lock"} {
		if _, ok := fsys.files[name]; ok {
			t.Errorf("%s written despite an error in an imported package", name)
		}
	}
	if len(res.Outputs) != 0 || len(res.Lock) != 0 {
		t.Errorf("outputs %+v, lock %+v", res.Outputs, res.Lock)
	}
	for _, f := range res.Check.Findings {
		if f.Package == "a" && f.Severity == canon.SeverityError {
			t.Errorf("the selected package has an error of its own: %+v", f)
		}
	}
	control := newMemFS(map[string][]byte{
		"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": []byte(importerPackage),
		"/law/b/b.canon": []byte(strings.Replace(brokenImport, `"not an int"`, "3", 1)),
	})
	cres, err := openTierProject(t, control).Build(context.Background(), canon.BuildOptions{Packages: []string{"a"}})
	if err != nil || cres.Check.HasErrors() {
		t.Fatalf("control build: %v, %+v", err, cres)
	}
	for _, name := range []string{"/law/out/a/tiers.json", "/law/a/canon.lock"} {
		if _, ok := control.files[name]; !ok {
			t.Errorf("control: %s not written with the import valid", name)
		}
	}
}

const layerFile = "package a\nlayer knights\n\namend tiers {\n  low.weight: 5\n}\n"

// API.md B1a: a build with layers never writes canon.lock; a plain build of the same files does.
func TestBuildWithLayersWritesNoLock(t *testing.T) {
	files := func() map[string][]byte {
		return map[string][]byte{
			"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": tierPackage("a"),
			"/law/a/knights.canon": []byte(layerFile),
		}
	}
	lockName := func(fsys *memFS) []string {
		var locks []string
		for name := range fsys.files { //canon:unordered collected into a list that is sorted
			if strings.HasSuffix(name, "canon.lock") {
				locks = append(locks, name)
			}
		}
		slices.Sort(locks)
		return locks
	}
	plain := newMemFS(files())
	if _, err := openTierProject(t, plain).Build(context.Background(), canon.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(lockName(plain)) == 0 {
		t.Fatal("a plain build wrote no canon.lock: the test proves nothing")
	}
	layered := newMemFS(files())
	p, err := canon.Open("/law", canon.Options{FS: layered, Cache: "off", Layers: []string{"knights"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Build(context.Background(), canon.BuildOptions{})
	if err != nil || res.Check.HasErrors() {
		t.Fatalf("layered build: %v, %+v", err, res)
	}
	if locks := lockName(layered); len(locks) != 0 || len(res.Lock) != 0 {
		t.Errorf("a build with layers wrote %v, lock changes %+v", locks, res.Lock)
	}
	if _, ok := layered.files["/law/out/a/tiers.json"]; !ok {
		t.Error("the layered build wrote no data output")
	}
}

// API.md B1b: a Target that is not go, cpp, ts, json, view or text is refused with *ValueError, even
// beside valid ones, and the refusal writes nothing.
func TestBuildRefusesUnknownTargetBesideValid(t *testing.T) {
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": tierPackage("a")})
	p := openTierProject(t, fsys)
	_, err := p.Build(context.Background(), canon.BuildOptions{Targets: []canon.Target{"json", "rust"}})
	var verr *canon.ValueError
	if !errors.Is(err, canon.ErrBadValue) || !errors.As(err, &verr) || verr.Got != `"rust"` {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := fsys.files["/law/out/a/tiers.json"]; ok {
		t.Error("a refused build wrote an output")
	}
}
