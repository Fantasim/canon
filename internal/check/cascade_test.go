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
		{"§11.1 input record reached from no value (progen E3022_7625, E3022_8766)", "local record Gen {\n  key: input String from env \"KEY\"\n  me: Gen = {}\n}\n", diag.E3022.Def()},
		{"§11.1 optional cycle", strings.Replace(gen, "%s", "?", 1), diag.E1903.Def()},
	})
}

// TYPES.md §2 and §1: an input field declared `T??` (E3401, recovered to `T?`) is not also E1910.
func TestInputOfDoubleOptional(t *testing.T) {
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
		{"two codes, both unknown", "local enum K @codes(UInt8) {\n  a = 0x\n  b = 0x\n}\n", diag.E1110.Def()},
		{"@cpp(defines:)", "local enum K @codes(UInt8) @cpp(defines: \"P\\q\") {\n  a = 1\n}\n", diag.E1109.Def()},
		{"@cpp(struct:, header:)", "local record R @cpp(struct: \"S\\q\", header: \"h\\q\") {\n  n: Int @cpp(field: \"m\\q\", type: \"T\\q\")\n}\n", diag.E1109.Def()},
		{"@go(name:), @cpp(name:)", "local record R @go(name: \"R\\q\") @cpp(name: \"R\\q\") {\n  n: Int\n}\n", diag.E1109.Def()},
		{"@json(tag:) and a case's field", "local variant V @json(tag: \"k\\q\") {\n  c {\n    k: Int @json(\"k\\q\")\n  }\n}\n", diag.E1109.Def()},
		{"@deprecated reason", "local record R {\n  n: Int @deprecated(\"x\\q\")\n}\n", diag.E1109.Def()},
	})
	out := checkBuilt(t, "package a\n\nlocal record R {\n  s: Int @json(none: \"a\\q\")\n}\n").out
	if want := "error[" + string(diag.E3316.Def().Code) + "]"; !strings.Contains(out, want) {
		t.Errorf("none: on a non-optional field is %s whatever its marker; got:\n%s", want, out)
	}
}

// TYPES.md §1, §7.4, §13.4, DECISIONS 215 (progen E1110_8010): a lexer error in a bound or an asset makes no E3316 or E3704.
func TestLexErrorInTypes(t *testing.T) {
	const pair = "local record Pair {\n  k: Int\n  v: Int\n}\n\n"
	runCascades(t, []cascadeCase{
		{"§7.4 pairs bound, invalid number", pair + "local record R {\n  ps: [Pair](..=06) = [] @json(pairs: [\"a{i}\", \"b{i}\"])\n}\n", diag.E1110.Def()},
		{"§7.4 unit on a Duration bound, invalid duration", "local record R {\n  d: Duration(..=1s1d)? = none @json(unit: s)\n}\n", diag.E1111.Def()},
		{"§13.4 asset root, invalid escape", "local type A = asset(\"@resource/I\\q\", ext: [dds])\n", diag.E1109.Def()},
		{"§13.4 asset extension, invalid escape", "local type A = asset(\"Icon\", ext: [\"d\\qs\"])\n", diag.E1109.Def()},
	})
}

// TYPES.md §5.2, §1, DECISIONS 215 (progen E1112_70091): a map key holding a lexer error is a string key (no E3305).
func TestLexErrorInMapKey(t *testing.T) {
	runCascades(t, []cascadeCase{
		{"empty interpolation", "const FAMILIES = {\n  \"{}IK3_YOBO\": [\"IK3_YOYO\"]\n}\n", diag.E1112.Def()},
	})
}

// TYPES.md §1, §3.2, WIRE.md §4.1: a field's own type expression decides its @json forms, in either declaration order.
func TestFieldTypeErrorOrderFree(t *testing.T) {
	e2102, e3002, e3015, e3316 := diag.E2102.Def().Code, diag.E3002.Def().Code, diag.E3015.Def().Code, diag.E3316.Def().Code
	for _, tc := range []struct {
		name, decl, rec string
		want            []diag.Code
	}{
		{"unit on an alias holding an error", "local type D = Duration | \"never\" | Nope2\n\n",
			"local record R {\n  d: D @json(unit: s)\n}\n\n", []diag.Code{e2102, e3002, e3002, e3316}},
		{"pairs over a record with an error field", "local record Pair {\n  k: Int\n  v: Int\n  w: Nope\n}\n\n",
			"local record R {\n  ps: [Pair](..=3) = [] @json(pairs: [\"a{i}\", \"b{i}\"])\n}\n\n", []diag.Code{e2102, e3316}},
		{"pairs bound reading a let", "local let lim = 3\n\nlocal record Pair {\n  k: Int\n  v: Int\n}\n\n",
			"local record R {\n  ps: [Pair](..=lim) = [] @json(pairs: [\"a{i}\", \"b{i}\"])\n}\n\n", []diag.Code{e3015}},
	} {
		before := checkBuilt(t, "package a\n\n"+tc.decl+tc.rec).codes
		after := checkBuilt(t, "package a\n\n"+tc.rec+tc.decl).codes
		if want := sortedCodes(tc.want); !slices.Equal(before, want) || !slices.Equal(after, want) {
			t.Errorf("%s: want %v in both orders, got %v declared first, %v declared last", tc.name, want, before, after)
		}
	}
}

// TYPES.md §1, WIRE.md §4.1, §5.14 (progen E3401_426): no @json form is judged on a field of the error type, but pairs: templates.
func TestJSONFormsOnErrorType(t *testing.T) {
	runCascades(t, []cascadeCase{
		{"§2 none: on T??", "local record R {\n  r: Int(0..)?? = none @json(none: \"=\")\n}\n", diag.E3401.Def()},
		{"inline", "local record R {\n  v: Nope @json(inline)\n}\n", diag.E2102.Def()},
		{"int", "local record R {\n  b: Nope @json(int)\n}\n", diag.E2102.Def()},
		{"bits", "local record R {\n  b: Nope @json(bits)\n}\n", diag.E2102.Def()},
		{"unit", "local record R {\n  d: Nope @json(unit: s)\n}\n", diag.E2102.Def()},
		{"pairs", "local record R {\n  ps: Nope @json(pairs: [\"a{i}\", \"b{i}\"])\n}\n", diag.E2102.Def()},
	})
	out := checkBuilt(t, "package a\n\nlocal record R {\n  ps: Nope @json(pairs: [\"a{i}\", \"a{i}\"])\n}\n").out
	if want := "error[" + string(diag.E3316.Def().Code) + "]"; !strings.Contains(out, want) {
		t.Errorf("two equal pairs: templates are %s whatever the field's type; got:\n%s", want, out)
	}
}

// LOCK.md §1, TYPES.md §1 (progen E3013_385): no @stable is judged off a stable table whose element is in error, nor on a field of the error type.
func TestStableOffALostTable(t *testing.T) {
	const rec = "local record Ev {\n  code: UInt16 @stable\n}\n\n"
	runCascades(t, []cascadeCase{
		{"an enum as the element", rec + "local enum K { a }\n\nlocal let evs: stable table K = {}\n", diag.E3013.Def()},
		{"unknown element", rec + "local let evs: stable table Nope = {}\n", diag.E2102.Def()},
		{"field type in error", "local record Ev {\n  code: Nope @stable\n}\n\nlocal let evs: stable table Ev = {}\n", diag.E2102.Def()},
	})
	got := checkBuilt(t, "package a\n\n"+rec+"local enum K { a }\n\nlocal let evs: table K = {}\n").codes
	if want := sortedCodes([]diag.Code{diag.E6003.Def().Code, diag.E3013.Def().Code}); !slices.Equal(got, want) {
		t.Errorf("a plain table lost nothing stable: want %v, got %v", want, got)
	}
}

// TYPES.md §1, WIRE.md §6.1: an expected type holding the error type anywhere gets no E7116.
func TestLoadIntoErrorType(t *testing.T) {
	runCascades(t, []cascadeCase{
		{"headered csv", "local let x: [Nope] = load.csv(\"a.csv\", header: true)\n", diag.E2102.Def()},
		{"dir of text", "local let x: [Nope] = load.dir(\"d/*.txt\", format: text)\n", diag.E2102.Def()},
		{"bare text", "local let x: [Nope] = load(\"x.txt\")\n", diag.E2102.Def()},
		{"plain csv", "local let x: [[Nope]] = load.csv(\"a.csv\")\n", diag.E2102.Def()},
		{"optional text", "local let x: Nope? = load(\"x.txt\")\n", diag.E2102.Def()},
		{"load.text", "local let x: Nope? = load.text(\"x.txt\")\n", diag.E2102.Def()},
	})
}

// TYPES.md §1, §9.1: a keyed list whose element is in error reports nothing of its key.
func TestKeyedErrorElement(t *testing.T) {
	const h = "local enum K { a }\n\nlocal record Ev {\n  k: K\n}\n\nlocal record H(e: Ev) {\n  n: Int\n}\n\n"
	runCascades(t, []cascadeCase{
		{"§11.1 parameterized", h + "local let hs: [H] keyed by n = []\n", diag.E3806.Def()},
		{"unknown", "local let xs: [Nope] keyed by n = []\n", diag.E2102.Def()},
	})
}

// GRAMMAR.md §8.3, TYPES.md §13.2: `@json(codes)` refused for want of `@codes` does not make a union's enum wire as codes.
func TestRefusedJSONCodes(t *testing.T) {
	runCascades(t, []cascadeCase{
		{"no @codes", "local enum T @json(codes) { a }\n\nlocal type U = T | \"all\"\n", diag.E1119.Def()},
	})
}
