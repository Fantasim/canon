package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// callsMain runs demo.calls' conformance vectors.
const callsMain = `#include "calls.gen.h"
#include <cstdio>
int main() {
    std::printf("failures: %d\n", demo::calls::conformance::RunCallsConformance());
    return 0;
}
`

// callsPackage is demo.calls, whose only package-fn call sits in a statement body (ir.Block)
// and names a fn declared after it, so its prototype must be written.
func callsPackage() *ir.Package {
	later := fnSpec{
		name: "later", params: params(tInt, "x"), result: tInt, body: bin(ir.OpAdd, tInt, param(0, tInt), lit(tInt, num(1))),
		vectors: [][]value.Value{vals(num(1), num(2))},
	}.build()
	x := param(0, tInt)
	caller := fnSpec{
		name: "caller", params: params(tInt, "x"), result: tInt,
		body: block(
			&ir.IfStmt{Cond: bin(ir.OpGt, tBool, x, lit(tInt, num(0))), Then: block(ret(&ir.CallFn{T: tInt, Fn: later, Args: []ir.PExpr{x}}))},
			ret(lit(tInt, num(0))),
		),
		vectors: [][]value.Value{vals(num(4), num(5)), vals(num(-1), num(0))},
	}.build()
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/calls/out", Mode: ir.ModeData, Namespace: "demo::calls"}
	return &ir.Package{Name: "demo.calls", Dir: "demo/calls", Fns: []*ir.ExportFn{caller, later}, Emits: []*ir.Emit{emit}}
}

// log-2026-09-24 (gen/cpp review calls): a package fn called only from a statement body still gets its prototype.
func TestBlockCallerCompilesAndRuns(t *testing.T) {
	dir := t.TempDir()
	files := generate(t, callsPackage())
	writeFiles(t, dir, files)
	if err := os.WriteFile(filepath.Join(dir, "main.cpp"), []byte(callsMain), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, out := range buildAndRun(t, dir, []string{"calls.gen.cpp", "calls_conformance.gen.cpp", "main.cpp"}) {
		if !strings.Contains(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
	}
}
