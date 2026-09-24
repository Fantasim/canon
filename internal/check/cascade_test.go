package check_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// only reports rendered findings whose errors are all of code d, at least one.
func only(out string, d *diag.Def) bool {
	n := strings.Count(out, "error[")
	return n > 0 && n == strings.Count(out, "error["+string(d.Code)+"]")
}

// cascadeCase is one program whose only errors must be of one code.
type cascadeCase struct {
	name, src string
	want      *diag.Def
}

func runCascades(t *testing.T, cases []cascadeCase) {
	t.Helper()
	for _, tc := range cases {
		out := checkBuilt(t, "package a\n\n"+tc.src).out
		if !only(out, tc.want) {
			t.Errorf("%s: want only %s, got:\n%s", tc.name, tc.want.Code, out)
		}
	}
}

// TYPES.md §10.2, §11.5, log 2026-09-24 (check C2 review): a let named in its own annotation is E3504.
func TestSelfNamingAnnotation(t *testing.T) {
	runCascades(t, []cascadeCase{
		{"§11.5 domain is the let itself", "local let t: {k in t: Int} = {}\n", diag.E3504.Def()},
		{"§11.5 domain through the let", "local let t: {k in t.x: Int} = {}\n", diag.E3504.Def()},
		{"§11.5 optional of it", "local let t: {k in t: Int}? = none\n", diag.E3504.Def()},
		{"§10.2 ref to itself", "local let t: [ref t] = []\n", diag.E3504.Def()},
		{"§1 a let of the error type", "local let t: Nope = 1\nlocal let u: [ref t] = []\nlocal let v: {k in t: Int} = {}\n", diag.E2102.Def()},
	})
	checkSource(t, `package a

local let t: table Item = { a {} }

local record Item {
  m: {k in t: Int} = {}
  r: ref t?
}

local let u: {k in t: Int} = {}
`)
}

// TYPES.md §1, §10.2: a ref that names no collection stands for the error type in every use.
func TestBrokenRefStandsForError(t *testing.T) {
	const uses = `local record Status {
  n: Int
  next: [ref %s]
}

local let statuses: table Status = { open { n: 1, next: [open] } }

local fn f(x: ref %s, y: ref statuses, xs: [ref %s]) -> Bool {
  let a = x == y
  let b = y == x
  let g = x.n + 1
  let h: Int = x.n
  let o = xs.first()!.next
  let q: [ref statuses] = xs
  let r: ref statuses = x
  let s = [x, y]
  let u = if a { x } else { y }
  return x == statuses.open
}
`
	var cases []cascadeCase
	for _, tc := range []struct {
		name, target string
		want         *diag.Def
	}{
		{"§3.3 unknown name", "Nope", diag.E2102.Def()},
		{"§10.2 no collection", "Status2", diag.E2103.Def()},
		{"§10.2 not a collection", "LIMIT", diag.E3504.Def()},
	} {
		src := "local record Status2 {\n  n: Int\n}\n\nlocal const LIMIT = 3\n\n" + strings.ReplaceAll(uses, "%s", tc.target)
		cases = append(cases, cascadeCase{tc.name, src, tc.want})
	}
	runCascades(t, cases)
}

// GRAMMAR.md §6.7, TYPES.md §12.2: a name given twice, a positional after a named: E1121, never E3004.
func TestRefusedArgumentsAreE1121s(t *testing.T) {
	const fn = "local fn f(a: Int, b: Int) -> Int {\n  return a + b\n}\n\n"
	runCascades(t, []cascadeCase{
		{"user fn, name twice", fn + "local let x: Int = f(a: 1, a: 2, b: 3)\n", diag.E1121.Def()},
		{"user fn, positional after named", fn + "local let x: Int = f(b: 1, 2)\n", diag.E1121.Def()},
		{"built-in method, name twice", `local let x: String = ["a"].join(sep: ",", sep: ";")` + "\n", diag.E1121.Def()},
		{"built-in method, positional after named", `local let x: String = ["a"].join(sep: ",", ";")` + "\n", diag.E1121.Def()},
		{"built-in fn, positional after named", "local let x: Int = max(b: 1, 2)\n", diag.E1121.Def()},
	})
}

// codesCase is one program and the exact error codes it reports, sorted.
type codesCase struct {
	name, src string
	want      []*diag.Def
}

func runCodes(t *testing.T, cases []codesCase) {
	t.Helper()
	for _, tc := range cases {
		b := checkBuilt(t, "package a\n\n"+tc.src)
		var want []diag.Code
		for _, d := range tc.want {
			want = append(want, d.Code)
		}
		if want = sortedCodes(want); !slices.Equal(b.codes, want) {
			t.Errorf("%s: codes %v, want %v:\n%s", tc.name, b.codes, want, b.out)
		}
	}
}

// TYPES.md §12.6, GRAMMAR.md §4.3, log 2026-09-24 (check C2 review): an E1126 member exists but is never missing.
func TestMatchLeavesOutUnmatchableMembers(t *testing.T) {
	const match = "local fn f(g: Grade) -> Bool {\n  return match g {\n    normal => true\n  }\n}\n"
	e1126, e3601 := diag.E1126.Def(), diag.E3601.Def()
	runCodes(t, []codesCase{
		{"only the none member missing", "local enum Grade { normal, none }\n\n" + match, []*diag.Def{e1126}},
		{"a none case, matched by kind", "local variant V { normal, none }\n\nlocal fn f(v: V) -> Bool {\n  return match v.kind {\n    normal => true\n  }\n}\n", []*diag.Def{e1126}},
		{"another member still missing", "local enum Grade { normal, other, none }\n\n" + match, []*diag.Def{e1126, e3601}},
		{"a lexer error in a member's annotation", "local enum Grade { normal, other @deprecated(\"x\\q\") }\n\n" + match, []*diag.Def{diag.E1109.Def(), e3601}},
		{"none as a pattern is the none literal", "local enum Grade { normal, none }\n\nlocal fn f(g: Grade) -> Bool {\n  return match g {\n    normal => true\n    none => false\n  }\n}\n",
			[]*diag.Def{e1126, diag.E3603.Def()}},
		{"the member exists: its missing code", "local enum K @codes(UInt8) { a = 1, none }\n", []*diag.Def{e1126, diag.E3201.Def()}},
		{"a sound enum", "local enum Grade { normal, other }\n\n" + match, []*diag.Def{e3601}},
	})
	if out := checkBuilt(t, "package a\n\nlocal enum Grade { normal, other, none }\n\n"+match).out; !strings.Contains(out, "cover other;") {
		t.Errorf("only other is missing:\n%s", out)
	}
}

// GRAMMAR.md §6.7, TYPES.md §12.4: a lambda no parameter types is still checked alone.
func TestLambdaCheckedAlone(t *testing.T) {
	const g = "local fn g(a: Int, f: fn(Int) -> Int) -> Int {\n  return f(a)\n}\n\n"
	e1121, e2102 := diag.E1121.Def(), diag.E2102.Def()
	runCodes(t, []codesCase{
		{"user fn, lambda after a named argument", g + "local let x: Int = g(a: 1, y => nope.n)\n", []*diag.Def{e1121, e2102}},
		{"built-in, lambda after a named argument", "local let x: [Int] = [1].filter(pred: y => y > 0, y => nope.n)\n", []*diag.Def{e1121, e2102}},
		{"unknown function", "local let x: Int = nope1(y => nope2.n)\n", []*diag.Def{e2102, e2102}},
		{"shorthand of the error type", "local let x: Int = nope1(.n)\n", []*diag.Def{e2102}},
		{"L1 a contextual name in the body", "local enum Grade { low, high }\n\nlocal fn g(a: Int, h: fn(Int) -> Grade) -> Grade {\n  return h(a)\n}\n\n" +
			"local let x: Grade = g(a: 1, y => low)\n", []*diag.Def{e1121}},
		{"L3 a name in the body of an unknown call's lambda", "local let x: Int = nope1(y => low)\n", []*diag.Def{e2102}},
		{"L4 none in the body", "local let x: Int = nope1(y => none)\n", []*diag.Def{e2102}},
		{"L5 an empty list in the body", "local let x: Int = nope1(y => [])\n", []*diag.Def{e2102}},
		{"an empty brace in the body", "local let x: Int = k(1, y => {})\n", []*diag.Def{e2102}},
	})
}

// TYPES.md §1, log 2026-09-24 (check C2 round-3): checked against the error type, nothing reports E3008.
func TestUnknownContextIsSilent(t *testing.T) {
	e2102 := diag.E2102.Def()
	runCodes(t, []codesCase{
		{"empty list", "local let x: Nope = []\n", []*diag.Def{e2102}},
		{"none", "local let x: Nope = none\n", []*diag.Def{e2102}},
		{"empty brace", "local let x: Nope = {}\n", []*diag.Def{e2102}},
		{"two literals", "local let x: Nope = 1 + 2\n", []*diag.Def{e2102}},
		{"a list holding none", "local let x: Nope = [none]\n", []*diag.Def{e2102}},
		{"a lambda", "local let x: Nope = y => []\n", []*diag.Def{e2102}},
		{"a list still checks its elements", "local let x: Nope = [nope.n]\n", []*diag.Def{e2102, e2102}},
	})
}

// DECISIONS 215, WIRE.md §4.2: only keys taken from an argument holding a lexer error are unknown.
func TestWireKeysOfOtherArguments(t *testing.T) {
	runCodes(t, []codesCase{
		{"bad none marker, known wire name", "local record R {\n  s: Int? @json(\"x\", none: \"a\\q\")\n  u: Int @json(\"x\")\n}\n",
			[]*diag.Def{diag.E1109.Def(), diag.E3316.Def()}},
		{"three pairs templates, one bad", "local record Pair {\n  k: Int\n  v: Int\n}\n\nlocal record R {\n  ps: [Pair](..=4) @json(pairs: [\"a{i}\", \"b{i}\", \"c{i}\\q\"])\n}\n",
			[]*diag.Def{diag.E1109.Def(), diag.E1119.Def(), diag.E3316.Def()}},
	})
}

// TYPES.md §1, EVALUATION.md §11.1: an input type in error anywhere is not also E1910.
func TestInputTypeInError(t *testing.T) {
	runCodes(t, []codesCase{
		{"unknown optional", "local record Gen {\n  key: input Nope? from env \"KEY\"\n}\n\nlocal record Cfg {\n  gen: Gen\n}\n\n/// The config.\nlet cfg: Cfg = { gen: {} }\n",
			[]*diag.Def{diag.E2102.Def()}},
	})
}

// TYPES.md §1, §3.5, §3.6: the reserved field reads as the error type only where it may be the entry's key.
func TestReservedFieldOnKeyedListElement(t *testing.T) {
	const both = "local record R {\n  id: Int = 0\n}\n\nlocal let kl: [R] keyed by id = [{ id: 1 }]\n\nlocal let t: table R = { a {} }\n\n"
	e2105 := diag.E2105.Def()
	runCodes(t, []codesCase{
		{"keyed-list element", both + "local let s: String = kl.first()!.id\n", []*diag.Def{e2105, diag.E3002.Def()}},
		{"table entry selected", both + "local let s: String = t.a.id\n", []*diag.Def{e2105}},
		{"ref into the table", both + "local let r: ref t = a\n\nlocal let s: String = r.id\n", []*diag.Def{e2105}},
	})
}

// TYPES.md §1, §3.6: a table element's field named id (E2105) reads as the error type.
func TestReservedEntryFieldReadsNothing(t *testing.T) {
	runCascades(t, []cascadeCase{
		{"§3.6 id", "local record R {\n  id: Int = 0\n}\n\nlocal let t: table R = { a {} }\n\nlocal let s: String = t.a.id\n", diag.E2105.Def()},
		{"§3.6 id through a lambda", "local record R {\n  id: Int = 0\n}\n\nlocal let t: table R = { a {} }\n\n" +
			"local let s: String = [t.a].map(.id).join(sep: \",\")\n", diag.E2105.Def()},
	})
}

// TYPES.md §1, §13.1, EVALUATION.md §11.1: E3022's cycle adds no E3302 and no path to an input record.
func TestSelfContainingRecordCascades(t *testing.T) {
	const gen = "local record Gen {\n  key: input String? from env \"KEY\"\n  me: Gen%s\n}\n\n" +
		"local record Cfg {\n  gen: Gen\n}\n\n/// The config.\nlet cfg: Cfg = { gen: {} }\n"
	runCascades(t, []cascadeCase{
		{"§13.1 literal", "local record P {\n  a: Int\n  me: P\n}\n\nlocal let p: P = { a: 1 }\n", diag.E3022.Def()},
		{"§13.1 through another record", "local record P {\n  q: Q\n}\n\nlocal record Q {\n  p: P\n}\n\nlocal let p: P = {}\n", diag.E3022.Def()},
		{"§11.1 input record", strings.Replace(gen, "%s", "", 1), diag.E3022.Def()},
		{"§11.1 optional cycle", strings.Replace(gen, "%s", "?", 1), diag.E1903.Def()},
	})
}

// TYPES.md §2 and §1: an input field whose type is in error (E3401) is not also E1910.
func TestInputOfErrorType(t *testing.T) {
	runCascades(t, []cascadeCase{
		{"§2 T??", "local record Gen {\n  key: input String?? from env \"KEY\"\n}\n\n" +
			"local record Cfg {\n  gen: Gen\n}\n\n/// The config.\nlet cfg: Cfg = { gen: {} }\n", diag.E3401.Def()},
	})
}

// DECISIONS 215: an enum member's value or an annotation argument holding a lexer error has the
// error type: no static check on the value the lexer made up.
func TestLexErrorOutsideExpressions(t *testing.T) {
	const pair = "local record Pair {\n  k: Int\n  v: Int\n}\n\n"
	runCascades(t, []cascadeCase{
		{"member value, string enum", "local enum K { a = 0x }\n", diag.E1110.Def()},
		{"member value, @codes enum", "local enum K @codes(UInt8) { a = \"x\\q\" }\n", diag.E1109.Def()},
		{"@json wire names", "local record R {\n  s: Int @json(\"p\\q\")\n  u: Int @json(\"p\\q\")\n}\n", diag.E1109.Def()},
		{"@json path", "local record R {\n  s: Int @json(path: \"a\\q\")\n}\n", diag.E1109.Def()},
		{"@json none marker", "local record R {\n  s: Int? @json(none: \"a\\q\")\n}\n", diag.E1109.Def()},
		{"@json pairs template", pair + "local record R {\n  ps: [Pair](..=4) @json(pairs: [\"a{i}\\q\", \"a{i}\\q\"])\n}\n", diag.E1109.Def()},
		{"emit package", "emit go {\n  out: \"gen\"\n  package: \"p\\q\"\n}\n", diag.E1109.Def()},
	})
	out := checkBuilt(t, "package a\n\nlocal record R {\n  s: Int @json(none: \"a\\q\")\n}\n").out
	if want := "error[" + string(diag.E3316.Def().Code) + "]"; !strings.Contains(out, want) {
		t.Errorf("none: on a non-optional field is %s whatever its marker; got:\n%s", want, out)
	}
}
