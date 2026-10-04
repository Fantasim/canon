package check_test

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

const checkPkg = "github.com/fantasim/canonlang/internal/check"

// binaryOperands is the operand count of a binary expression.
const binaryOperands = 2

// checkSource checks one file of package a and fails on any finding.
func checkSource(t *testing.T, src string) (*check.Program, *syntax.File) {
	t.Helper()
	prog, file, out := checkFile(t, src)
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	return prog, file
}

// checkFile checks one file of package a and renders every finding, the parser's included.
func checkFile(t *testing.T, src string) (*check.Program, *syntax.File, string) {
	t.Helper()
	fs := &source.FileSet{}
	parse := diag.NewBag(fs, "")
	f, err := fs.Add("a/a.canon", "/a/a.canon", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	file := syntax.Parse(f, syntax.FileSource, parse)
	bags := check.Bags{}
	prog := check.Check(context.Background(), exampleProject(), []*syntax.File{file}, bags, literalFolder{})
	return prog, file, render(t, fs, parse, bags)
}

// nodesOf lists the nodes of type N in a file, in source order.
func nodesOf[N syntax.Node](f *syntax.File) []N {
	var out []N
	syntax.Inspect(f, func(n syntax.Node) bool {
		if x, ok := n.(N); ok {
			out = append(out, x)
		}
		return true
	})
	return out
}

// TYPES.md §6.2, DECISIONS: parentheses are transparent, the conversion sits on the innermost node.
func TestParensAreTransparent(t *testing.T) {
	prog, f := checkSource(t, `package a

local fn f(x: Int?) -> Float {
  let n: Int? = ((none))
  if ((x != none)) {
    return ((1))
  }
  return 0.5
}
`)
	for _, p := range nodesOf[*syntax.ParenExpr](f) {
		if prog.Info.Conv[p] != nil {
			t.Errorf("conversion recorded on a parenthesis at %v", p.First())
		}
		if prog.Info.Types[p] == nil {
			t.Errorf("no type for the parenthesis at %v", p.First())
		}
	}
	for _, n := range nodesOf[*syntax.NoneLit](f) {
		if prog.Info.Types[n] == nil {
			t.Errorf("no type for the inner none at %v", n.First())
		}
	}
	lit := nodesOf[*syntax.IntLit](f)[0]
	if cv := prog.Info.Conv[lit]; cv == nil || cv.Kind != check.ConvIntLitToFloat {
		t.Errorf("Conv[1] = %v, want IntLitToFloat on the literal", cv)
	}
}

// TYPES.md §7.5, DECISIONS 154: an optional ref compared with an entry is dereferenced.
func TestOptionalRefDeref(t *testing.T) {
	prog, f := checkSource(t, `package a

local record Flag {
  label: String
}

local let flags: table Flag = {
  blocking { label: "b" }
}

local fn same(r: (ref flags)?, e: Flag) -> Bool {
  return r == e
}
`)
	bin := nodesOf[*syntax.BinaryExpr](f)[0]
	cv := prog.Info.Conv[bin.X]
	if cv == nil || cv.Kind != check.ConvPresent || cv.Inner == nil || cv.Inner.Kind != check.ConvDeref {
		t.Errorf("Conv[r] = %+v, want Present over Deref", cv)
	}
}

// STDLIB.md §1.1: a type parameter of a built-in is bound per call and never reaches Info.
func TestNoTypeParameterInInfo(t *testing.T) {
	prog, _ := loadExamples(t).run(t)
	for _, msg := range leakedTypes(prog.Info) {
		t.Error(msg)
	}
}

// STDLIB.md §1.1: String(x)'s callee records its signature bound to x's type (bug B1).
func TestStringCalleeIsInstantiated(t *testing.T) {
	prog, f := checkSource(t, "package a\n\n/// F.\nexport fn f(n: Int) -> String { return String(n) }\n")
	for _, msg := range leakedTypes(prog.Info) {
		t.Error(msg)
	}
	call := nodesOf[*syntax.CallExpr](f)[0]
	if got, want := fmt.Sprint(prog.Info.Types[call.Fun]), "fn(Int) -> String"; got != want {
		t.Errorf("Types[String] = %s, want %s", got, want)
	}
}

// leakedTypes describes every type of info holding a checker-private type.
func leakedTypes(info *check.Info) []string {
	var all []types.Type
	for _, typ := range info.Types {
		all = append(all, typ)
	}
	for _, typ := range info.TypeExprs {
		all = append(all, typ)
	}
	for _, sel := range info.Selections {
		all = append(all, sel.Recv)
	}
	for _, cv := range info.Conv {
		all = append(all, cv.From, cv.To)
	}
	for _, obj := range info.Defs {
		all = append(all, obj.Type())
	}
	seen := map[uintptr]bool{}
	var out []string
	for _, typ := range all {
		if name := leaked(reflect.ValueOf(typ), seen); name != "" {
			out = append(out, fmt.Sprintf("%v holds a %s", typ, name))
		}
	}
	return out
}

// leaked names the checker-private type found in v, or "".
func leaked(v reflect.Value, seen map[uintptr]bool) string {
	switch v.Kind() {
	case reflect.Interface:
		return leaked(v.Elem(), seen)
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return ""
		}
		seen[v.Pointer()] = true
		if el := v.Type().Elem(); el.PkgPath() == checkPkg {
			return el.Name()
		}
		return leaked(v.Elem(), seen)
	case reflect.Struct:
		for i := range v.NumField() {
			if name := leaked(v.Field(i), seen); name != "" {
				return name
			}
		}
	case reflect.Slice:
		for i := range v.Len() {
			if name := leaked(v.Index(i), seen); name != "" {
				return name
			}
		}
	default:
	}
	return ""
}

// TYPES.md §5.1, STDLIB.md §5: a bare name in `in`, contains and indexOf is a key of the collection.
func TestBareKeysInMembership(t *testing.T) {
	prog, f := checkSource(t, `package a

local record Flag {
  label: String
}

local let flags: table Flag = {
  blocking { label: "b" }
  sensitive { label: "s" }
  muted { label: "m" }
}

local fn keys() -> Bool {
  return blocking in flags and flags.contains(sensitive) and flags.indexOf(muted) != none
}
`)
	want := map[string]bool{"blocking": true, "sensitive": true, "muted": true}
	for _, id := range nodesOf[*syntax.IdentExpr](f) {
		if !want[id.Name] {
			continue
		}
		if coll := prog.Info.Keys[id]; coll == nil || coll.Name != "flags" {
			t.Errorf("Keys[%s] = %v, want flags", id.Name, coll)
		}
		delete(want, id.Name)
	}
	if len(want) != 0 {
		t.Errorf("keys never seen: %v", want)
	}
}

// TYPES.md §1: a node the parser already reported gives the error type, with no second finding.
func TestNoCascadeAfterSyntaxError(t *testing.T) {
	_, _, out := checkFile(t, "package a\n\nlocal record E {\n  n: Int\n}\n\nlocal type P(e: E) = Int\n\nlocal record R {\n  e: E\n  p: P(*)\n}\n")
	if !hasOnly(out, diag.E1116.Def()) {
		t.Errorf("a type argument the parser refused: want only %s, got:\n%s", diag.E1116.Def().Code, out)
	}
	for _, decl := range []string{
		"local let r: Int = max(1, /a/)",
		`local let r: Bool = "a".matches((/a/))`,
	} {
		_, _, out := checkFile(t, "package a\n\n"+decl+"\n")
		if !hasOnly(out, diag.E1115.Def()) {
			t.Errorf("%s: want only %s, got:\n%s", decl, diag.E1115.Def().Code, out)
		}
	}
}

// EVALUATION.md §3.2: a cycle between consts is eval's E4301; check breaks them without a finding.
func TestConstCycleLeftToEval(t *testing.T) {
	prog, _ := checkSource(t, `package a

local const A = B + 1
local const B = A + 1
local const C = 2
`)
	broken := map[string]bool{}
	for _, d := range prog.Packages[0].Decls {
		broken[d.Name()] = prog.Info.Broken[d]
	}
	if !broken["A"] || !broken["B"] || broken["C"] {
		t.Errorf("broken = %v, want A and B only", broken)
	}
}

// IMPLEMENTATION-PLAN §4.7: Check needs its bags and its Folder; without them it returns nil.
func TestCheckNeedsBagsAndFolder(t *testing.T) {
	ctx := context.Background()
	if check.Check(ctx, exampleProject(), nil, nil, literalFolder{}) != nil {
		t.Error("nil bags: want nil")
	}
	if check.Check(ctx, exampleProject(), nil, check.Bags{}, nil) != nil {
		t.Error("nil fold: want nil")
	}
}

// EVALUATION.md §11.1: E1903 counts paths per record, so 2^40 paths through 40 levels is quick.
func TestInputPathsAreCountedOnce(t *testing.T) {
	const levels = 40
	for _, tc := range []struct {
		fields string
		want   bool
	}{
		{"  a: L%d?\n", false},
		{"  a: L%d?\n  b: L%d?\n", true},
	} {
		var b strings.Builder
		b.WriteString("package a\n\n/// The config.\nlet cfg: L0 = {}\n\n")
		for i := range levels {
			fmt.Fprintf(&b, "local record L%d {\n"+strings.ReplaceAll(tc.fields, "%d", strconv.Itoa(i+1))+"}\n\n", i)
		}
		fmt.Fprintf(&b, "local record L%d {\n  key: input String? from env \"KEY\"\n}\n", levels)
		start := time.Now()
		_, _, out := checkFile(t, b.String())
		if got := hasOnly(out, diag.E1903.Def()); got != tc.want {
			t.Errorf("%q: %s = %v, want %v:\n%s", tc.fields, diag.E1903.Def().Code, got, tc.want, out)
		}
		if d := time.Since(start); d > time.Second {
			t.Errorf("%q: took %v", tc.fields, d)
		}
	}
}

// CODEGEN.md §2.1, GRAMMAR.md §2.6: an interpolated emit string is E1132, reported once.
func TestEmitInterpolationOnce(t *testing.T) {
	_, _, out := checkFile(t, "package a\n\nlocal let x: Int = 1\n\nemit go {\n  out: \"gen/{x}.go\"\n}\n")
	if !hasOnly(out, diag.E1132.Def()) {
		t.Errorf("want one %s, got:\n%s", diag.E1132.Def().Code, out)
	}
}

// hasOnly reports rendered findings that are exactly one error, of code d.
func hasOnly(out string, d *diag.Def) bool {
	return strings.Count(out, "error[") == 1 && strings.Contains(out, "["+string(d.Code)+"]")
}

// SPEC §1, WIRE.md §5.14: checking `pairs:` keys does not depend on the length bound's magnitude.
func TestPairsBoundIsFree(t *testing.T) {
	start := time.Now()
	_, _, out := checkFile(t, `package a

local record Pair {
  k: Int
  v: Int
}

local record R {
  params: [Pair](..=1_000_000_000) @json(pairs: ["a{i}", "b{i}"])
  other: [Pair](..=1_000_000_000) @json(pairs: ["c{i}", "d{i}"])
}
`)
	if !strings.HasPrefix(out, noFindings) {
		t.Errorf("findings:\n%s", out)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("took %v", d)
	}
}

// TYPES.md §5.1, §5.3 (log 2026-09-24, W1 dependent types): in `let f: Float = 1 / 2` each literal takes Float, so the division is Float's.
func TestContextArithmeticInfo(t *testing.T) {
	prog, f := checkSource(t, "package a\n\nlocal let f: Float = 1 / 2\n")
	bin := nodesOf[*syntax.BinaryExpr](f)[0]
	if got := prog.Info.Types[bin]; got == nil || got.Kind() != types.Float {
		t.Errorf("Types[1 / 2] = %v, want Float", got)
	}
	lits := nodesOf[*syntax.IntLit](f)
	if len(lits) != binaryOperands {
		t.Fatalf("%d integer literals, want one per operand", len(lits))
	}
	for _, lit := range lits {
		if cv := prog.Info.Conv[lit]; cv == nil || cv.Kind != check.ConvIntLitToFloat {
			t.Errorf("Conv[%v] = %v, want IntLitToFloat", lit.Value, cv)
		}
	}
}
