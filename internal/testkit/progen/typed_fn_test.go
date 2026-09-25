package progen_test

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	fnName      = "calc"
	probePrefix = "probe"
	fnMaxDepth  = 3  // the operators an Int expression nests: 50^8 stays far inside int64
	fnLitMax    = 20 // the largest literal operand
	probeMax    = 50 // the largest argument a probe passes
	probeCount  = 3
	maxParams   = 2
)

// fnParamNames are the export fn's parameters, in order.
var fnParamNames = []string{"a", "b"}

// intOps are the portable subset's Int operators but / and %, which a probe could divide by zero.
var intOps = []string{"+", "-", "*"}

// cmpOps are the comparisons an `if` condition uses (CONFORMANCE.md §2.2).
var cmpOps = []string{"<", "<=", ">", ">=", "==", "!="}

// intFuncs are the Int built-ins of the portable subset taking two arguments.
var intFuncs = []string{"min", "max"}

// typedFn is a package-level translated export fn and the calls it is probed with (CODEGEN.md §5.10).
type typedFn struct {
	params            []string
	cond, then, other string
	probes            [][]int
}

// genFn is a seeded fn whose every operator reads a parameter on one side (TYPES.md §5.1, E3008).
func genFn(r *progen.Rand) typedFn {
	fn := typedFn{params: fnParamNames[:1+r.Intn(maxParams)]}
	fn.cond = genIntExpr(r, fn.params, 1) + " " + progen.Pick(r, cmpOps) + " " + genIntExpr(r, fn.params, 1)
	fn.then = genIntExpr(r, fn.params, fnMaxDepth)
	fn.other = genIntExpr(r, fn.params, fnMaxDepth)
	for range probeCount {
		args := make([]int, len(fn.params))
		for i := range args {
			args[i] = r.Intn(2*probeMax+1) - probeMax
		}
		fn.probes = append(fn.probes, args)
	}
	return fn
}

// genIntExpr is an Int expression over params, nested at most depth operators deep, that reads
// at least one parameter.
func genIntExpr(r *progen.Rand, params []string, depth int) string {
	if depth == 0 || r.OneIn(3) {
		return progen.Pick(r, params)
	}
	x := genIntExpr(r, params, depth-1)
	switch r.Intn(4) {
	case 0:
		return progen.Pick(r, intFuncs) + "(" + x + ", " + genIntExpr(r, params, depth-1) + ")"
	case 1:
		return "abs(" + x + ")"
	}
	y := strconv.Itoa(r.Intn(fnLitMax + 1))
	if r.OneIn(2) {
		y = genIntExpr(r, params, depth-1)
	}
	if r.OneIn(2) {
		x, y = y, x
	}
	return "(" + x + " " + progen.Pick(r, intOps) + " " + y + ")"
}

// renderFn writes the fn and one public value per probe, holding the evaluator's answer.
func renderFn(b *strings.Builder, fn typedFn) []string {
	params := make([]string, len(fn.params))
	for i, p := range fn.params {
		params[i] = p + ": Int"
	}
	fmt.Fprintf(b, "/// %s.\nexport fn %s(%s) -> Int {\n", fnName, fnName, strings.Join(params, ", "))
	fmt.Fprintf(b, "  if %s {\n    return %s\n  }\n  return %s\n}\n", fn.cond, fn.then, fn.other)
	names := make([]string, 0, len(fn.probes))
	for i, args := range fn.probes {
		name := fmt.Sprintf("%s%d", probePrefix, i)
		fmt.Fprintf(b, "let %s: Int = %s(%s)\n", name, fnName, joinInts(args))
		names = append(names, name)
	}
	return names
}

// joinInts is ns as a comma-separated argument list.
func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

// probeStmts are the smoke test's comparisons of the generated fn with the evaluator: each probe's
// Go result against the value the build's JSON holds for it (read by the smoke's probe helper).
func probeStmts(fn typedFn) []string {
	out := make([]string, 0, len(fn.probes))
	for i, args := range fn.probes {
		call := fmt.Sprintf("%s.%s(%s)", typedPackage, goName(fnName), joinInts(args))
		out = append(out, fmt.Sprintf("if got, want := strconv.FormatInt(%s, 10), probe(t, %q); got != want { t.Errorf(%q, got, want) }",
			call, fmt.Sprintf("%s%d", probePrefix, i), smokeMark+"fn "+call+" = %s, the evaluator's %s"))
	}
	return out
}

// probeHelper is the smoke test's reader of a probe's JSON value (WIRE.md §8.1), as its text.
const probeHelper = `
func probe(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "data", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Value json.Number }
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	return doc.Value.String()
}
`
