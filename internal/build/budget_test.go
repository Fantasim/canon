package build_test

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/fnsize"
)

// budgetPackage is one package of the budget tree: its mode, its source and whether it has a TS emit.
type budgetPackage struct {
	pkg, mode string
	ts        bool
	source    string
}

// budgetPackages is project (a) in three modes and project (b) in three shapes (CODEGEN.md §2.11).
func budgetPackages() []budgetPackage {
	var pkgs []budgetPackage
	for _, mode := range []string{modeTypes, modeData, modeBaked} {
		pkg := "w" + mode
		pkgs = append(pkgs, budgetPackage{pkg, mode, true, wideSource(pkg, mode)})
	}
	// idata has no lookup fn: a data-mode lookup fn is E8013.
	return append(pkgs,
		budgetPackage{"cbaked", modeBaked, true, codesSource("cbaked", modeBaked, false, true)},
		budgetPackage{"ibaked", modeBaked, false, codesSource("ibaked", modeBaked, true, true)},
		budgetPackage{"idata", modeData, false, codesSource("idata", modeData, true, false)},
		budgetPackage{"ctypes", modeTypes, true, codesSource("ctypes", modeTypes, false, false)},
		budgetPackage{"otypes", modeTypes, true, openCodesSource("otypes")})
}

// budgetTree is the project holding the packages.
func budgetTree(pkgs []budgetPackage) mapFS {
	fsys := mapFS{"p/project.canon": file(outProject)}
	for _, p := range pkgs {
		fsys["p/"+p.pkg+"/"+p.pkg+".canon"] = file(p.source)
	}
	return fsys
}

// measured is one generated function with the file it is in.
type measured struct {
	File   string
	Target ir.Target
	fnsize.Func
}

// measure returns every function of the generated files of the build, runtime files and the
// TypeScript helper block left out.
func measure(t *testing.T, outs []build.Output) []measured {
	t.Helper()
	var all []measured
	for _, o := range outs {
		funcs, err := measureOutput(o)
		if err != nil {
			t.Fatalf("%s: %v", o.Path, err)
		}
		for _, f := range funcs {
			all = append(all, measured{o.Path, o.Target, f})
		}
	}
	return all
}

var (
	errUnknownSuffix = errors.New("a file of a measured target has no known suffix")
	errNoFunction    = errors.New("no function found in a measured file")
)

// runtimeFiles are the files every package of Go and C++ gets beside its own code, below its output.
var runtimeFiles = []string{"go/rt/rt.go", "cpp/canon_runtime.h", "cpp/canon_runtime_json.h"}

// measurers parse the files of each measured target; a TypeScript file is measured without its helper block.
var measurers = map[ir.Target]func(string) ([]fnsize.Func, error){
	ir.TargetGo: needFunctions(fnsize.Go), ir.TargetCpp: needFunctions(fnsize.Cpp),
	ir.TargetTS: func(src string) ([]fnsize.Func, error) { return fnsize.TS(stripHelperBlock(src)) },
}

// measureOutput measures a file of a Go, C++ or TypeScript output; a file it cannot measure, or a Go
// or C++ file in which it finds no function, is an error, and the runtime files are skipped by path.
func measureOutput(o build.Output) ([]fnsize.Func, error) {
	measurer, ok := measurers[o.Target]
	if !ok || slices.Contains(runtimeFiles, strings.TrimPrefix(o.Path, "@out/"+outputPackage(o.Path)+"/")) {
		return nil, nil
	}
	if !slices.ContainsFunc([]string{".go", ".h", ".cpp", ".ts"}, func(ext string) bool { return strings.HasSuffix(o.Path, ext) }) {
		return nil, errUnknownSuffix
	}
	return measurer(string(o.Content))
}

// needFunctions makes a measurer fail on a source in which it finds no function.
func needFunctions(measurer func(string) ([]fnsize.Func, error)) func(string) ([]fnsize.Func, error) {
	return func(src string) ([]fnsize.Func, error) {
		funcs, err := measurer(src)
		if err == nil && len(funcs) == 0 {
			err = errNoFunction
		}
		return funcs, err
	}
}

var separator = regexp.MustCompile(`(?m)^// -{20,}$`)

// stripHelperBlock drops the TypeScript helper block, banner to banner (CODEGEN.md §8.2).
func stripHelperBlock(src string) string {
	idx := separator.FindAllStringIndex(src, -1)
	const bannerLines = 3
	if len(idx) < bannerLines {
		return src
	}
	return src[:idx[0][0]] + src[idx[bannerLines-1][0]:]
}

// constructs name what a function is, by the first pattern its name matches; the maximum of each
// target, mode and construct is logged.
var constructs = []struct {
	name string
	re   *regexp.Regexp
}{
	{"decode", regexp.MustCompile(`(^|::|\.)[dD]ecode`)},
	{"build", regexp.MustCompile(`(^|::|\.)[bB]uild`)},
	{"inputs", regexp.MustCompile(`LoadInputs`)},
	{"make", regexp.MustCompile(`[mM]ake`)},
	{"lookup", regexp.MustCompile(`(?i)^` + lookupFnName + `$`)},
	{"enum", regexp.MustCompile(`^(IsRetired|ToName|ToWire|ToCode|IsKnown|Parse\w+)$|(\.|^)(String|Wire|Known|Retired)$|FromWire$|FromCode$|Members$`)},
}

// otherConstruct is the construct of a function no pattern of constructs names.
const otherConstruct = "other"

func constructOf(name string) string {
	for _, c := range constructs {
		if c.re.MatchString(name) {
			return c.name
		}
	}
	return otherConstruct
}

// outputPackage is the package a display path "@out/<pkg>/..." belongs to.
func outputPackage(display string) string {
	return strings.Split(strings.TrimPrefix(display, "@out/"), "/")[0]
}

// targetNames name the targets of the budget in logs.
var targetNames = map[ir.Target]string{ir.TargetGo: "go", ir.TargetCpp: "cpp", ir.TargetTS: "ts"}

// budgetTargets are the targets the budget measures.
var budgetTargets = []ir.Target{ir.TargetGo, ir.TargetCpp, ir.TargetTS}

// budgetBuild builds the tree, for the targets given or every one, and returns its outputs; nothing
// is written.
func budgetBuild(t *testing.T, fsys mapFS, targets ...ir.Target) []build.Output {
	t.Helper()
	res := buildTree(t, fsys, build.BuildOptions{Check: true, Targets: targets})
	if res.Summary.Errors != 0 {
		t.Fatalf("the budget tree has errors: %v", codes(res.List))
	}
	return res.Outputs
}

// CODEGEN.md §2.11, D-SIZE (DECISIONS 435): no generated function of project (a) or (b) passes ir.PartLines unless it is one statement.
func TestGeneratedFunctionsStayInTheBudget(t *testing.T) {
	pkgs := budgetPackages()
	modeOf, wantModes := map[string]string{}, map[string]bool{}
	for _, p := range pkgs {
		modeOf[p.pkg] = p.mode
		for _, target := range budgetTargets {
			if target != ir.TargetTS || p.ts {
				wantModes[targetNames[target]+" "+p.mode] = true
			}
		}
	}
	maxima, seen, counts := map[string]measured{}, map[string]bool{}, map[string]int{}
	outs := budgetBuild(t, budgetTree(pkgs))
	for _, path := range expectedPaths(pkgs) {
		if !slices.ContainsFunc(outs, func(o build.Output) bool { return o.Path == path }) {
			t.Errorf("the build wrote no %s", path)
		}
	}
	for _, m := range measure(t, outs) {
		pkg := outputPackage(m.File)
		mode := targetNames[m.Target] + " " + modeOf[pkg]
		seen[mode] = true
		key := mode + " " + constructOf(m.Name)
		if cur, ok := maxima[key]; !ok || m.Body > cur.Body {
			maxima[key] = m
		}
		for _, shape := range partShapes {
			if shape.pkg == pkg && shape.re.MatchString(m.Name) {
				counts[shape.name]++
			}
		}
		if m.Body > ir.PartLines && m.Statements != 1 {
			t.Errorf("%s: %s has a body of %d lines in %d statements, over the budget of %d", m.File, m.Name, m.Body, m.Statements, ir.PartLines)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(maxima)) {
		t.Logf("max %-22s %5d lines  %s %s", k, maxima[k].Body, maxima[k].File, maxima[k].Name)
	}
	for _, mode := range slices.Sorted(maps.Keys(wantModes)) {
		if !seen[mode] {
			t.Errorf("no function of %q was measured", mode)
		}
	}
	for _, shape := range partShapes {
		if counts[shape.name] < shape.least {
			t.Errorf("%s: %d parts, want at least %d", shape.name, counts[shape.name], shape.least)
		}
	}
}

// expectedPaths are the files every package must write: its Go file, its C++ header and source, and
// its TypeScript file when it has one.
func expectedPaths(pkgs []budgetPackage) []string {
	var paths []string
	for _, p := range pkgs {
		base := "@out/" + p.pkg + "/"
		paths = append(paths, base+"go/"+p.pkg+".gen.go", base+"cpp/"+p.pkg+".gen.h", base+"cpp/"+p.pkg+".gen.cpp")
		if p.ts {
			paths = append(paths, base+p.pkg+".ts")
		}
	}
	return paths
}

// partShapes are the cuts project (a) must show: a budget met by leaving a function out would pass
// the bound above.
var partShapes = []struct {
	name, pkg string
	re        *regexp.Regexp
	least     int
}{
	{"C++ types Decode parts", "w" + modeTypes, regexp.MustCompile(`Access::Decode\d+$`), 2},
	{"Go baked build parts", "w" + modeBaked, regexp.MustCompile(`^build\w+_\d+$`), 90},
	{"C++ baked Build parts", "w" + modeBaked, regexp.MustCompile(`Access::Build\d+$`), 45},
}

// edge is a function whose size the boundary test sets to the line: its name uncut and its parts, and
// source, a package with so many units of a big and a small kind; twoKinds says the small exists.
type edge struct {
	name     string
	target   ir.Target
	whole    string
	parts    *regexp.Regexp
	source   func(big, small int) string
	twoKinds bool
}

// probeUnits is the number of units of the two probe builds that measure a weight.
const probeUnits = 10

// edges are the cut functions: the build and the decoder of each of Go and C++.
var edges = []edge{
	{"go build", ir.TargetGo, "buildEdge", regexp.MustCompile(`^buildEdge_\d+$`),
		func(big, _ int) string { return buildEdgeSource(big) }, false},
	{"cpp build", ir.TargetCpp, "EdgeAccess::Build", regexp.MustCompile(`^EdgeAccess::Build\d+$`),
		func(big, _ int) string { return buildEdgeSource(big) }, false},
	{"go decode", ir.TargetGo, "decodeW", regexp.MustCompile(`^decodeW_\d+$`), decodeEdgeSource, true},
	{"cpp decode", ir.TargetCpp, "Decode", regexp.MustCompile(`^EdgeAccess::Decode\d+$`), decodeEdgeSource, true},
}

// mix splits total into n units of big lines and m of small ones with m < big, the smallest m.
func mix(total, big, small int) (n, m int, ok bool) {
	if big <= 0 {
		return 0, 0, false
	}
	for m = 0; m < big; m++ {
		if rest := total - small*m; rest >= 0 && rest%big == 0 {
			return rest / big, m, true
		}
	}
	return 0, 0, false
}

// build writes the package with the units and returns the one function e.whole and the parts of it.
func (e edge) build(t *testing.T, big, small int) (whole measured, parts []measured) {
	t.Helper()
	fsys := mapFS{"p/project.canon": file(outProject), "p/" + edgePkg + "/" + edgePkg + ".canon": file(e.source(big, small))}
	var found []measured
	for _, m := range measure(t, budgetBuild(t, fsys, e.target)) {
		switch {
		case m.Name == e.whole:
			found = append(found, m)
		case e.parts.MatchString(m.Name):
			parts = append(parts, m)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d functions named %s, want one", len(found), e.whole)
	}
	return found[0], parts
}

// body is the lines of the whole function for the units, which must not be cut.
func (e edge) body(t *testing.T, big, small int) int {
	t.Helper()
	whole, parts := e.build(t, big, small)
	if len(parts) != 0 {
		t.Fatalf("%d units of one kind and %d of the other are already cut", big, small)
	}
	return whole.Body
}

// weights measures, from probes of probeUnits and one more unit, the lines a function holds besides
// its units and the lines of each kind of unit.
func (e edge) weights(t *testing.T) (base, big, small int) {
	t.Helper()
	first := e.body(t, probeUnits, 0)
	big = e.body(t, probeUnits+1, 0) - first
	base = first - big*probeUnits
	if e.twoKinds {
		first = e.body(t, 0, probeUnits)
		small = e.body(t, 0, probeUnits+1) - first
		if first != base+small*probeUnits {
			t.Fatalf("the base differs with the kind of unit: %d, want %d", first, base+small*probeUnits)
		}
	}
	return base, big, small
}

// CODEGEN.md §2.11, DECISIONS 435: a body of exactly ir.PartLines lines is not cut, one more is.
func TestPartLinesIsTheExactBoundary(t *testing.T) {
	for _, e := range edges {
		base, big, small := e.weights(t)
		for _, lines := range []int{ir.PartLines - 1, ir.PartLines, ir.PartLines + 1} {
			t.Run(fmt.Sprintf("%s %d", e.name, lines), func(t *testing.T) {
				n, m, ok := mix(lines-base, big, small)
				if !ok {
					t.Fatalf("%d lines cannot be made of units of %d and %d lines over a base of %d", lines, big, small, base)
				}
				whole, parts := e.build(t, n, m)
				if lines <= ir.PartLines {
					if len(parts) != 0 || whole.Body != lines {
						t.Errorf("%s holds %d lines in %d parts, want %d lines and no part", e.whole, whole.Body, len(parts), lines)
					}
					return
				}
				if len(parts) < 1 || whole.Body >= lines {
					t.Errorf("%d lines: %d parts, %s still holds %d lines", lines, len(parts), e.whole, whole.Body)
				}
				for _, p := range parts {
					t.Logf("%s holds %d lines", p.Name, p.Body)
					if p.Body > ir.PartLines {
						t.Errorf("%s holds %d lines, over the budget", p.Name, p.Body)
					}
				}
			})
		}
	}
}
