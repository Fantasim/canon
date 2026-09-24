package eval_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
)

// codeOf is a code of the registry as text, for a comparison.
func codeOf(c interface{ Def() *diag.Def }) string {
	return string(c.Def().Code)
}

// prelude declares the values the rule cases use, so no operand is a bare literal (TYPES.md §5.1).
const prelude = `
local let zero: Int = 0
local let two: Int = 2
local let seven: Int = 7
local let big: Int = 9223372036854775807
local let half: Float = 0.5
local let xs: [Int] = [3, 1, 2]
local let words: [String] = ["b", "a", "c"]
local let nums: {String: Int} = { "a": 1, "b": 2 }

/// A color.
enum Color ordered { red, green, blue }

/// A pet.
record Pet {
  /// Its name.
  name: String
  /// Its age.
  age: Int = 1
  /// Its nickname.
  nick: String = "{name}!"
}

/// Pets.
let pets: table Pet = {
  rex { name: "Rex", age: 3 }
  tom { name: "Tom" }
  retired old { name: "Old", age: 9 }
}
`

// evalCase is one rule: an expression of a type, and the text of its value or the codes of
// the findings it produces.
type evalCase struct {
	rule, typ, expr, want string
}

// evaluate evaluates `local let x: typ = expr` after the prelude; the result is x's canonical
// text, or the codes of the findings when x is poisoned.
func evaluate(t *testing.T, typ, expr string, opt eval.Options) string {
	t.Helper()
	src := "/// A.\npackage a\n" + prelude + "\nlocal let x: " + typ + " = " + expr + "\n"
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), opt)
	if v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]; ok {
		return v.CanonText()
	}
	var codes []string
	for _, bag := range b.bags {
		for _, f := range bag.Findings() {
			codes = append(codes, string(f.Code))
		}
	}
	for _, f := range b.prog.parse.Findings() {
		codes = append(codes, string(f.Code))
	}
	return strings.Join(codes, " ")
}

func runCases(t *testing.T, cases []evalCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.rule+": "+c.expr, func(t *testing.T) {
			if got := evaluate(t, c.typ, c.expr, eval.Options{}); got != c.want {
				t.Errorf("%s = %q, want %q", c.expr, got, c.want)
			}
		})
	}
}

func TestIntegers(t *testing.T) {
	runCases(t, []evalCase{
		{"EVALUATION.md §6.1 truncation", "Int", "-seven / two", "-3"},
		{"EVALUATION.md §6.1 remainder sign", "Int", "-seven % two", "-1"},
		{"EVALUATION.md §6.1 MIN % -1", "Int", "(-big - 1) % -1", "0"},
		{"EVALUATION.md §6.1 MIN / -1", "Int", "(-big - 1) / -1", codeOf(diag.E4101)},
		{"EVALUATION.md §6.1 overflow of *", "Int", "big * two", codeOf(diag.E4101)},
		{"EVALUATION.md §6.1 overflow of unary -", "Int", "-(-big - 1)", codeOf(diag.E4101)},
		{"EVALUATION.md §6.1 zero divisor of %", "Int", "seven % zero", codeOf(diag.E4102)},
		{"EVALUATION.md §6.1 a..=MAX", "Range", "0..=big", codeOf(diag.E4101)},
		{"TYPES.md §13.3 a..=b stores b + 1", "Range", "0..=seven", "0..8"},
	})
}

func TestFloats(t *testing.T) {
	runCases(t, []evalCase{
		{"EVALUATION.md §6.2 0/0", "Float", "(half - half) / (half - half)", codeOf(diag.E4104)},
		{"EVALUATION.md §6.2 -0.0 == 0.0", "Bool", "-0.0 == half - half", "true"},
		{"EVALUATION.md §6.2 Int(f) truncates", "Int", "Int(-half - 2.0)", "-2"},
		{"STDLIB.md §2.2 round half away from zero", "Int", "round(-half - 2.0)", "-3"},
		{"STDLIB.md §2.2 floor", "Int", "floor(-half)", "-1"},
		{"STDLIB.md §2.2 ceil", "Int", "ceil(half)", "1"},
		{"STDLIB.md §2.2 sqrt of a negative", "Float", "sqrt(-half)", codeOf(diag.E4104)},
		{"STDLIB.md §2.2 pow overflow", "Float", "pow(10.0, 400.0)", codeOf(diag.E4104)},
		{"STDLIB.md §2.2 min orders -0.0 first", "Float", "min(half - half, -0.0)", "0"},
		{"STDLIB.md §9.2 canonical float", "String", `"{half * 1e30}"`, "5e+29"},
		{"TYPES.md §7.3 Float32 rounding", "Float32", "half / 3.0", "0.16666667"},
		{"TYPES.md §6.2 integer literal to Float", "Float", "3", "3"},
	})
}

func TestDurations(t *testing.T) {
	runCases(t, []evalCase{
		{"EVALUATION.md §6.3 Duration / Int", "Duration", "7ms / two", "3ms"},
		{"EVALUATION.md §6.3 Duration / Duration", "Float", "90s / 1m", "1.5"},
		{"EVALUATION.md §6.3 Int * Duration", "Duration", "two * 45s", "1m30s"},
		{"EVALUATION.md §6.3 zero divisor", "Duration", "1s / zero", codeOf(diag.E4102)},
		{"STDLIB.md §9.3 duration text", "String", `"{48h + 1500ms}"`, "2d1s500ms"},
	})
}

func TestOperators(t *testing.T) {
	runCases(t, []evalCase{
		{"TYPES.md §7.1 string +", "String", `words[0] + words[1]`, "ba"},
		{"TYPES.md §7.1 list +", "[Int]", "xs + [seven]", "[3, 1, 2, 7]"},
		{"TYPES.md §7.5 ordered enum", "Bool", "Color.red < Color.blue", "true"},
		{"TYPES.md §7.5 strings by bytes", "Bool", `words[0] < words[1]`, "false"},
		{"TYPES.md §7.1 in a list", "Bool", "seven in xs", "false"},
		{"TYPES.md §7.1 in a range", "Bool", "two in 0..3", "true"},
		{"STDLIB.md §5 a key in a table", "Bool", `"rex" in pets`, "true"},
		{"TYPES.md §7.1 in a map", "Bool", `"b" in nums`, "true"},
		{"TYPES.md §6.5 ?? on none", "Int", "xs.get(9) ?? seven", "7"},
		{"TYPES.md §6.5 x! on none", "Int", "xs.get(9)!", codeOf(diag.E4001)},
		{"TYPES.md §6.5 ?. skips the chain", "Int?", "pets.get(\"zz\")?.age", "none"},
		{"EVALUATION.md §2.2 and is lazy", "Bool", "false and seven / zero == 1", "false"},
		{"EVALUATION.md §2.2 or is lazy", "Bool", "true or seven / zero == 1", "true"},
	})
}

func TestCollections(t *testing.T) {
	runCases(t, []evalCase{
		{"STDLIB.md §4.1 negative index", "Int", "xs[-1]", "2"},
		{"STDLIB.md §4.1 index out of range", "Int", "xs[-4]", codeOf(diag.E4002)},
		{"STDLIB.md §4.1 slice", "[Int]", "xs[1..]", "[1, 2]"},
		{"STDLIB.md §4.1 slice with negative bound", "[Int]", "xs[..-1]", "[3, 1]"},
		{"STDLIB.md §4.1 slice by a Range value", "[Int]", "xs[0..=1]", "[3, 1]"},
		{"STDLIB.md §7 string slice", "String", `"héllo"[0..1]`, "h"},
		{"STDLIB.md §5 entry by key", "Int", "pets.rex.age", "3"},
		{"STDLIB.md §5 missing key", "Int", `pets["zz"].age`, codeOf(diag.E4002)},
		{"STDLIB.md §3 .id and .retired", "String", `"{pets.old.id} {pets.old.retired}"`, "old true"},
		{"TYPES.md §6.2 table to list", "Int", "pets.active().len()", "2"},
		{"STDLIB.md §6 map lookup", "Int", `nums["b"]`, "2"},
		{"STDLIB.md §6 missing map key", "Int", `nums["z"]`, codeOf(diag.E4002)},
		{"TYPES.md §15 defaults see earlier fields", "String", "pets.tom.nick", "Tom!"},
		{"EVALUATION.md §2.2 comprehension", "[Int]", "[x * two for x in xs if x > 1]", "[6, 4]"},
		{"EVALUATION.md §2.2 map comprehension", "{String: Int}", `{w: two for w in words}`, `{"b": 2, "a": 2, "c": 2}`},
		{"TYPES.md §5.2 spread then fields", "String", `"{Pet { ...pets.rex, age: 5 }}"`, `Pet{name: "Rex", age: 5, nick: "Rex!"}`},
		{"TYPES.md §5.2 a duplicate map key keeps the first", "{String: Int}", `{ words[0]: 1, words[0]: 2 }`, `{"b": 1}`},
	})
}

func TestRefinements(t *testing.T) {
	runCases(t, []evalCase{
		{"TYPES.md §7.4 range", "Int(0..5)", "seven", "7"},
		{"TYPES.md §7.2 sized integer", "UInt8", "seven * 100", "700"},
		{"TYPES.md §7.4 pattern", "String(/^a/)", "words[0]", "b"},
		{"TYPES.md §7.4 where", "Int where it > 10", "seven", "7"},
		{"TYPES.md §7.4 length of a list", "[Int](..2)", "xs", "[3, 1, 2]"},
	})
}

// EVALUATION.md §4.3, §7.1: soft findings at a storage point; the let is still evaluated.
func TestRefinementFindings(t *testing.T) {
	for _, c := range []struct{ typ, expr, code string }{
		{"Int(0..5)", "seven", codeOf(diag.E3204)},
		{"UInt8", "seven * 100", codeOf(diag.E3201)},
		{"String(/^a/)", "words[0]", codeOf(diag.E3205)},
		{"Int where it > 10", "seven", codeOf(diag.E3206)},
		{"Float32", "1e300 * half", codeOf(diag.E3202)},
		{"Duration", "9223372036854ms * two", codeOf(diag.E3201)},
		{"{String: Int}", "{ words[0]: 1, words[0]: 2 }", codeOf(diag.E3322)},
	} {
		t.Run(c.code, func(t *testing.T) {
			src := "/// A.\npackage a\n" + prelude + "\nlocal let x: " + c.typ + " = " + c.expr + "\n"
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{})
			if out := b.findings(t); !strings.Contains(out, "["+c.code+"]") {
				t.Errorf("findings:\n%s", out)
			}
		})
	}
}

func TestStrings(t *testing.T) {
	runCases(t, []evalCase{
		{"STDLIB.md §7 split", "[String]", `"a,,b".split(",")`, `["a", "", "b"]`},
		{"STDLIB.md §7 split of empty", "[String]", `words[0][0..0].split(",")`, `[""]`},
		{"STDLIB.md §7 find empty", "Int?", `"x".find("")`, "0"},
		{"STDLIB.md §7 trim", "String", "\" \\t x \\n\".trim()", "x"},
		{"STDLIB.md §7 lower ASCII only", "String", `"ÉTÉ Ab".lower()`, "ÉtÉ ab"},
		{"STDLIB.md §7 replace", "String", `"aaa".replace("a", "bb")`, "bbbbbb"},
		{"STDLIB.md §8 matches is a search", "Bool", `"xaby".matches(/ab/)`, "true"},
		{"STDLIB.md §7 len counts bytes", "Int", `"é".len()`, "2"},
		{"STDLIB.md §9.4 nested strings quoted", "String", `"{words}"`, `["b", "a", "c"]`},
		{"STDLIB.md §9.1 String(x)", "String", "String(Color.green)", "green"},
	})
}

func TestFormatSpecs(t *testing.T) {
	runCases(t, []evalCase{
		{"STDLIB.md §9.5 ,", "String", `"{seven * 176367:,}"`, "1,234,569"},
		{"STDLIB.md §9.5 .N on an Int", "String", `"{seven:.2}"`, "7.00"},
		{"STDLIB.md §9.5 tie away from zero", "String", `"{half / 4.0:.2}"`, "0.13"},
		{"STDLIB.md §9.5 exact binary value", "String", `"{2.675 * (half + half):.2}"`, "2.67"},
		{"STDLIB.md §9.5 +", "String", `"{zero:+}"`, "+0"},
		{"STDLIB.md §9.5 negative zero digits", "String", `"{-half / 500.0:.2}"`, "0.00"},
		{"STDLIB.md §9.5 ,.2", "String", `"{-1234.5 * (half + half):,.2}"`, "-1,234.50"},
	})
}

func TestSequences(t *testing.T) {
	runCases(t, []evalCase{
		{"STDLIB.md §4.2 map", "[Int]", "xs.map(x => x * two)", "[6, 2, 4]"},
		{"STDLIB.md §4.2 filter", "[Int]", "xs.filter(x => x > 1)", "[3, 2]"},
		{"STDLIB.md §4.2 flatMap", "[Int]", "xs.flatMap(x => [x, x])", "[3, 3, 1, 1, 2, 2]"},
		{"STDLIB.md §4.2 flatten", "[Int]", "[xs, xs].flatten()", "[3, 1, 2, 3, 1, 2]"},
		{"STDLIB.md §4.2 reverse", "[Int]", "xs.reverse()", "[2, 1, 3]"},
		{"STDLIB.md §4.5 sortBy is stable", "[String]", `["bb", "a", "cc", "d"].sortBy(.len())`, `["a", "d", "bb", "cc"]`},
		{"STDLIB.md §4.2 unique", "[Int]", "(xs + xs).unique()", "[3, 1, 2]"},
		{"STDLIB.md §4.2 enumerate", "String", `"{words.enumerate()[1]}"`, `(1, "a")`},
		{"STDLIB.md §4.2 pairs", "Int", "xs.pairs().len()", "3"},
		{"STDLIB.md §4.2 zip", "String", `"{xs.zip(words)[0]}"`, `(3, "b")`},
		{"STDLIB.md §4.2 intersect", "[Int]", "xs.intersect([2, 3, 3])", "[3, 2]"},
		{"STDLIB.md §4.2 union", "[Int]", "xs.union([4, 3, 4])", "[3, 1, 2, 4]"},
		{"STDLIB.md §4.2 diff", "[Int]", "xs.diff([1])", "[3, 2]"},
		{"STDLIB.md §4.2 groupBy first-seen order", "String", `"{xs.groupBy(x => x % two)}"`, "{1: [3, 1], 0: [2]}"},
		{"STDLIB.md §4.2 toMap", "{String: Int}", "words.toMap(w => w, w => w.len())", `{"b": 1, "a": 1, "c": 1}`},
		{"STDLIB.md §4.2 join", "String", `words.join("-")`, "b-a-c"},
		{"STDLIB.md §4.3 any", "Bool", "xs.any(x => x > 2)", "true"},
		{"STDLIB.md §4.3 all", "Bool", "xs.all(x => x > 2)", "false"},
		{"STDLIB.md §4.3 count", "Int", "xs.count(x => x > 1)", "2"},
		{"STDLIB.md §4.3 isUnique", "Bool", "(xs + [1]).isUnique()", "false"},
		{"STDLIB.md §4.3 sum", "Int", "xs.sum()", "6"},
		{"STDLIB.md §4.3 sum overflow", "Int", "[big, big].sum()", codeOf(diag.E4101)},
		{"STDLIB.md §4.3 min", "Int?", "xs.min()", "1"},
		{"STDLIB.md §4.3 max of empty", "Int?", "xs[0..0].max()", "none"},
		{"STDLIB.md §4.3 minBy first of ties", "String?", `words.minBy(w => w.len())`, "b"},
		{"STDLIB.md §4.3 maxBy", "String?", `words.maxBy(w => w)`, "c"},
		{"STDLIB.md §4.1 first(pred)", "Int?", "xs.first(x => x < 3)", "1"},
		{"STDLIB.md §4.1 last", "Int?", "xs.last()", "2"},
		{"STDLIB.md §4.1 indexOf", "Int?", "xs.indexOf(2)", "2"},
		{"STDLIB.md §4.1 get out of range", "Int?", "xs.get(3)", "none"},
	})
}

func TestKeyedAndMaps(t *testing.T) {
	runCases(t, []evalCase{
		{"STDLIB.md §5 get", "Int?", `pets.get("tom")?.age`, "1"},
		{"STDLIB.md §5 at", "String", "pets.at(-1).name", "Old"},
		{"STDLIB.md §5 at out of range", "String", "pets.at(3).name", codeOf(diag.E4002)},
		{"STDLIB.md §5 keys", "String", `"{pets.keys()}"`, "[rex, tom, old]"},
		{"STDLIB.md §5 active", "Int", "pets.active().len()", "2"},
		{"STDLIB.md §6 keys and values", "String", `"{nums.keys()} {nums.values()}"`, `["a", "b"] [1, 2]`},
		{"STDLIB.md §6 map values", "{String: Int}", "nums.map(v => v * two)", `{"a": 2, "b": 4}`},
		{"STDLIB.md §6 filter takes (k, v)", "{String: Int}", `nums.filter((k, v) => v > 1)`, `{"b": 2}`},
		{"STDLIB.md §6 count", "Int", `nums.count((k, v) => k == "a")`, "1"},
		{"STDLIB.md §10 range len", "Int", "(two..seven).len()", "5"},
		{"STDLIB.md §10 open range len", "Int", "(two..).len()", codeOf(diag.E4002)},
	})
}

func TestGraphs(t *testing.T) {
	graph := "[1, 2, 3, 4]"
	runCases(t, []evalCase{
		{"STDLIB.md §2.3 reachable preorder", "[Int]", "reachable(from: 1, next: n => [n * two, n + 3].filter(m => m < 8))", "[1, 2, 4, 7, 5]"},
		{"STDLIB.md §2.3 reachable, one successor", "[Int]", "reachable(from: 1, next: n => if n < 4 { n + 1 } else { none })", "[1, 2, 3, 4]"},
		{"STDLIB.md §2.3 topoSort", "[Int]", "topoSort(" + graph + ", next: n => if n > 1 { [n - 1] } else { [] })", "[1, 2, 3, 4]"},
		{"STDLIB.md §2.3 topoSort input order", "[Int]", "topoSort([3, 1, 2], next: n => xs[0..0])", "[3, 1, 2]"},
		{"STDLIB.md §2.3 topoSort cycle", "[Int]", "topoSort(" + graph + ", next: n => [n % 4 + 1])", codeOf(diag.E4501)},
		{"STDLIB.md §2.3 cycles self-loop", "[Int]", "cycles(" + graph + ", next: n => if n == 2 { [2] } else { [] })", "[2]"},
		{"STDLIB.md §2.3 cycles component", "[Int]", "cycles(" + graph + ", next: n => [n % 3 + 1])", "[1, 2, 3]"},
	})
}

func TestMath(t *testing.T) {
	runCases(t, []evalCase{
		{"STDLIB.md §2.2 abs", "Int", "abs(-seven)", "7"},
		{"STDLIB.md §2.2 abs of MIN", "Int", "abs(-big - 1)", codeOf(diag.E4101)},
		{"STDLIB.md §2.2 min of several", "Int", "min(seven, two, 5)", "2"},
		{"STDLIB.md §2.2 max of durations", "Duration", "max(1s, 2m, 3ms)", "2m"},
		{"STDLIB.md §2.2 clamp", "Int", "clamp(seven, 0, 5)", "5"},
		{"STDLIB.md §2.2 clamp lo > hi", "Int", "clamp(seven, 5, 0)", codeOf(diag.E4108)},
		{"STDLIB.md §2.1 Float(i)", "Float", "Float(seven) / 2.0", "3.5"},
		{"STDLIB.md §2.1 Int(f) out of range", "Int", "Int(1e19 * half * 4.0)", codeOf(diag.E4103)},
	})
}

func TestControl(t *testing.T) {
	runCases(t, []evalCase{
		{"TYPES.md §12.6 match on an enum", "Int", "match Color.green { red => 1, green => 2, blue => 3 }", "2"},
		{"TYPES.md §12.6 match with _", "Int", "match Color.blue { red => 1, _ => 9 }", "9"},
		{"TYPES.md §12.6 match on Bool", "String", `match seven > two { true => "yes", false => "no" }`, "yes"},
		{"TYPES.md §6.5 if expression", "Int", "if seven > 10 { 1 } else if seven > 5 { 2 } else { 3 }", "2"},
		{"TYPES.md §8.1 enum members", "String", `"{Color.blue.index} {Color.blue.name}"`, "2 blue"},
	})
}

// xs3 is a list of three, four steps.
const xs3 = "local let xs: [Int] = [3, 1, 2]\n"

// EVALUATION.md §12.1, §12.2: exact costs, seen at the budget whose last step they spend.
func TestStepCosts(t *testing.T) {
	for _, c := range []struct {
		rule, src string
		steps     int64
	}{
		{"a literal, then a name", "local let two: Int = 2\nlocal let x: Int = two", 2},
		{"a binary operator and its operands", "local let two: Int = 2\nlocal let x: Int = two + two", 4},
		{"a method call: name, .f, call, and the built-in's 1", "local let xs: [Int] = [3, 1, 2]\nlocal let x: Int = xs.len()", 8},
		{"a shorthand: invocation, implicit parameter, postfix steps", "local let ws: [String] = [\"a\"]\nlocal let x: [Int] = ws.map(.len())", 12},
		{"a user call: name, call, invocation, statement, body", "local fn f(n: Int) -> Int { return n }\nlocal let x: Int = f(3)", 6},
		{"STDLIB.md §4.5 sortBy: 3 comparisons", xs3 + "local let x: [Int] = xs.sortBy(x => x)", 4 + 13},
		{"STDLIB.md §4.1 first(pred): the elements visited", xs3 + "local let x: Int? = xs.first(x => x > 5)", 4 + 19},
		{"STDLIB.md §9.1 String(x): the values visited", xs3 + "local let x: String = String(xs)", 4 + 7},
		{"STDLIB.md §9.1 interpolation: the values visited", xs3 + "local let x: String = \"{xs}\"", 4 + 6},
		{"STDLIB.md §4.1 a slice: its elements", xs3 + "local let x: [Int] = xs[0..2]", 4 + 7},
		{"STDLIB.md §4.2 pairs: one per pair", xs3 + "local let x: Int = xs.pairs().len()", 4 + 9},
		{"STDLIB.md §4.2 flatMap: n and the result", xs3 + "local let x: [Int] = xs.flatMap(x => [x])", 4 + 19},
		{"STDLIB.md §2.3 reachable: nodes and successors", "local let x: [Int] = reachable(from: 1, next: n => if n < 3 { [n + 1] } else { [] })", 33},
		{"STDLIB.md §2.3 topoSort: nodes and successors", xs3 + "local let x: [Int] = topoSort(xs, next: n => xs[0..0])", 4 + 25},
	} {
		t.Run(c.rule, func(t *testing.T) {
			src := []byte("/// A.\npackage a\n\n" + c.src + "\n")
			for budget, want := range map[int64]bool{c.steps: true, c.steps + 1: false} {
				b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{src}), eval.Options{Budget: budget})
				if got := strings.Contains(b.findings(t), "["+codeOf(diag.E4401)+"]"); got != want {
					t.Errorf("budget %d: exhausted %t, want %t\n%s", budget, got, want, b.findings(t))
				}
			}
		})
	}
}

// DECISIONS 185: a built-in charges an element as it takes it, so sum's overflow at the second
// element comes before the budget runs out on the third.
func TestChargeBeforeError(t *testing.T) {
	src := []byte("/// A.\npackage a\n\nlocal let big: Int = 9223372036854775807\nlocal let x: Int = [big, big, 1, 1, 1].sum()\n")
	for budget, want := range map[int64]string{11: codeOf(diag.E4401), 12: codeOf(diag.E4101), 100: codeOf(diag.E4101)} {
		b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{src}), eval.Options{Budget: budget})
		if out := b.findings(t); !strings.Contains(out, "["+want+"]") {
			t.Errorf("budget %d: want %s\n%s", budget, want, out)
		}
	}
}
