package ir_test

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	headerSuffix     = ".gen.h"
	cppSourceSuffix  = ".cpp"
	cppRuntimePrefix = "canon_runtime"
)

// TestCppNamePlanMatchesGeneratedCpp is decision 37 (CODEGEN.md §3.3–§3.5; log-2026-09-24 "ir plans review"): the C++ name plan of every data-mode cpp emit has no problem where gen/cpp generates the package, and holds exactly the names its header, .gen.cpp and conformance file declare, scope by scope; the plan's scopes and problems are the golden.
func TestCppNamePlanMatchesGeneratedCpp(t *testing.T) {
	golden.Run(t, "testdata/cppplan/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != planFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		var b strings.Builder
		for _, p := range w.build(t) {
			for _, e := range p.Emits {
				if e.Target == ir.TargetCpp {
					fakeVectors(p)
					pl := ir.PlanCppNames(p, e)
					b.WriteString(dumpScopes(ir.CppScopeNames(pl), pl.Problems()))
					compareCpp(t, p, e, pl)
				}
			}
		}
		b.WriteString(w.findings(t))
		return []byte(b.String())
	}, golden.Expected(planFile))
}

// compareCpp checks that gen/cpp generates the package and that its files declare, scope by scope (the namespace, detail, conformance, each class, struct and enum), exactly the plan's names.
func compareCpp(t *testing.T, p *ir.Package, e *ir.Emit, pl *ir.CppNamePlan) {
	t.Helper()
	files, err := cppgen.Generate(p, e)
	if err != nil || len(pl.Problems()) > 0 {
		t.Fatalf("%s: gen/cpp says %v, the plan %+v", p.Name, err, pl.Problems())
	}
	scan := newCppScan(e.Namespace)
	for _, f := range files {
		if strings.HasSuffix(f.Path, cppSourceSuffix) || strings.HasSuffix(f.Path, headerSuffix) && !strings.HasPrefix(path.Base(f.Path), cppRuntimePrefix) {
			scan.file(string(f.Content))
		}
	}
	want := cppPlanned(p, pl)
	scopes := maps.Clone(scan.names)
	maps.Copy(scopes, want)
	for _, sc := range slices.Sorted(maps.Keys(scopes)) {
		sameNames(t, p.Name+" "+sc, scan.names[sc], want[sc])
	}
}

// dumpScopes prints a plan's scopes, each with its names sorted, then its problems.
func dumpScopes(scopes map[string]map[string]bool, problems []ir.GoNameProblem) string {
	var b strings.Builder
	for _, sc := range slices.Sorted(maps.Keys(scopes)) {
		fmt.Fprintf(&b, "%s: %s\n", sc, strings.Join(slices.Sorted(maps.Keys(scopes[sc])), " "))
	}
	for _, pr := range problems {
		fmt.Fprintf(&b, "problem %d %s %s: %s, %s\n", pr.Kind, pr.Scope, pr.Name, pr.First, pr.Origin)
	}
	return b.String()
}

// TestSharedNamespaceUnselected is CODEGEN.md §3.5 (log-2026-09-24 "ir plans review"): a C++ namespace holds every package the build loads into it, an imported package that is not selected included, so `check b` reports at b the class Tone that b and its import a both declare in namespace sov::gen.
func TestSharedNamespaceUnselected(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte("package a\n\n/// A tone.\nenum Tone { soft, loud }\n\n/// A rank.\nenum Rank { low, high }\n\nemit cpp { out: \"@features/a\", namespace: \"sov::gen\", mode: data }\n"))
	w.add(t, "b/b.canon", []byte("package b\n\nimport a { Rank }\n\n/// A tone.\nrecord Tone {\n  /// Its rank.\n  rank: Rank\n}\n\nemit cpp { out: \"@features/b\", namespace: \"sov::gen\", mode: data }\n"))
	w.calls = w.fixtureCalls
	w.build(t, "b")
	out := w.findings(t)
	code := "[" + string(diag.E8005.Def().Code) + "]"
	if !strings.Contains(out, code+"  b/b.canon") || !strings.Contains(out, "a.Tone and b.Tone") {
		t.Errorf("want %s at b for a.Tone and b.Tone:\n%s", code, out)
	}
}
