package ir_test

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

const (
	mxCanonExt   = ".canon"
	mxCppSuffix  = ".gen.cpp"
	mxParallel   = 2
	mxErrorLines = 6
	mxMaxDepth   = 3
)

// mxCases are the legal combinations of leaf, depth and mode, and how many the matrix leaves out.
func mxCases(goGen bool) (cases []mxCase, illegal int) {
	for _, leaf := range mxLeaves {
		for depth := 1; depth <= mxMaxDepth; depth++ {
			cases, illegal = mxCasesOf(cases, illegal, leaf, depth, goGen)
		}
	}
	return cases, illegal
}

// mxCasesOf adds the legal modes of a leaf at a depth to cases.
func mxCasesOf(cases []mxCase, illegal int, leaf mxLeaf, depth int, goGen bool) ([]mxCase, int) {
	for _, mode := range mxModes {
		c := mxCase{leaf: leaf, depth: depth, mode: mode, goGen: goGen}
		if !c.legal() {
			illegal++
			continue
		}
		cases = append(cases, c)
	}
	return cases, illegal
}

// mxWrite writes a project of the library and the files into a new directory and returns it.
func mxWrite(t *testing.T, goGen bool, files []mxFile) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, e2eProjFile), []byte(e2eFeatures), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	for _, f := range append(mxLibFiles(goGen), files...) {
		pkg := filepath.Join(dir, f.pkg)
		if err := os.MkdirAll(pkg, e2eDirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkg, f.pkg+mxCanonExt), []byte(f.text), e2eFileMode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// mxProbe is one position of one case, written alone: the check says whether Canon refuses it.
type mxProbe struct {
	c   mxCase
	pos mxPos
}

// mxRefusal is a position Canon's check refused, with the first error it gave.
type mxRefusal struct {
	c       mxCase
	pos     mxPos
	code    diag.Code
	message string
}

// mxRefusals checks every position of every case alone and returns, by case name, the positions Canon accepts, and the ones it refuses.
func mxRefusals(t *testing.T, goGen bool, cases []mxCase) (kept map[string][]mxPos, refused []mxRefusal) {
	t.Helper()
	kept = map[string][]mxPos{}
	var probes []mxProbe
	var files []mxFile
	for _, c := range cases {
		for _, p := range mxPositions {
			c.tag = p.code
			probes = append(probes, mxProbe{c, p})
			files = append(files, c.files([]mxPos{p})...)
		}
	}
	probeDir := mxWrite(t, goGen, files)
	bad := mxCheckErrors(t, probeDir)
	if err := os.RemoveAll(probeDir); err != nil {
		t.Fatal(err)
	}
	for _, pr := range probes {
		base := pr.c
		base.tag = ""
		var first *mxFinding
		for _, f := range pr.c.files(nil) {
			if got, ok := bad[f.pkg]; ok {
				first = &got
			}
		}
		if first == nil {
			kept[base.name()] = append(kept[base.name()], pr.pos)
			continue
		}
		refused = append(refused, mxRefusal{c: base, pos: pr.pos, code: first.code, message: first.message})
	}
	return kept, refused
}

// mxFinding is the first error of a package.
type mxFinding struct {
	code    diag.Code
	message string
}

// mxCheckErrors checks the project in dir and returns, by package, its first error.
func mxCheckErrors(t *testing.T, dir string) map[string]mxFinding {
	t.Helper()
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	bad := map[string]mxFinding{}
	for _, f := range res.List {
		if _, seen := bad[f.Package]; f.Severity == diag.Error && !seen {
			bad[f.Package] = mxFinding{f.Code, f.Message}
		}
	}
	return bad
}

// mxBuild builds the project in dir and requires no finding.
func mxBuild(t *testing.T, dir string) {
	t.Helper()
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Errors > 0 {
		t.Fatalf("build findings:\n%s", renderFindings(t, res.Findings))
	}
}

// mxCompile compiles each source with cc under cxx.Flags, mxParallel at a time, and returns the first lines of the errors of each that fails, by source.
func mxCompile(t *testing.T, cc, include, gen string, sources []string) map[string]string {
	t.Helper()
	failed := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, mxParallel)
	for _, src := range sources {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			args := slices.Concat(cxx.Flags, []string{"-I", include, "-I", gen, "-c", "-o", os.DevNull, src})
			out, err := cxx.Compile(exec.Command(cc, args...))
			if err != nil {
				mu.Lock()
				failed[src] = mxFirstErrors(string(out))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return failed
}

// mxFirstErrors are the first lines of a compiler's output that say error.
func mxFirstErrors(out string) string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "error") && len(lines) < mxErrorLines {
			lines = append(lines, "    "+l)
		}
	}
	return strings.Join(lines, "\n")
}

// mxSources are the .gen.cpp of the build in dir, in path order.
func mxSources(t *testing.T, dir string) []string {
	t.Helper()
	return filesUnder(t, filepath.Join(dir, e2eGenRoot), mxCppSuffix)
}

// TestCppRefMatrix is CODEGEN.md §5.3, §5.8, §5.9 and DECISIONS 387, 456 compiled, once for every combination Canon accepts of a leaf kind a chain of refs ends in (an enum whose wire is not its name, an @codes enum, a String, an Int, a table's ids of the package or of another), a depth of 1, 2 and 3 refs, a position (a field, a list element, a map key, a literal union, a dependent type's branch) and a mode (baked, data with and without @reload, types, a data and a types reader of another package's class): with -Werror, on one compiler. Canon's check decides what is accepted, strictly: a refusal must be on the pinned allowlist (mxAllowed), the positions each case keeps are pinned (mxPinned), and so is the number of packages compiled. A combination that does not compile is a bug of gen/cpp's.
func TestCppRefMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	compilers, include := cxx.Toolchain(t)
	m := mxPrepare(t, false)
	if m.dir == "" {
		return // nothing is compiled of a matrix that moved
	}
	sources := mxSources(t, m.dir)
	if len(sources) != mxPackages {
		t.Errorf("%d packages compiled (the library included), %d pinned", len(sources), mxPackages)
	}
	failed := mxCompile(t, compilers[0], include, filepath.Join(m.dir, e2eGenRoot), sources)
	t.Logf("%d cases (%d illegal: an own table's ids are an enum in a baked package only), %d positions refused by the allowlist, %d packages compiled, %d fail", m.cases, m.illegal, m.refused, len(sources), len(failed))
	mxCheckPending(t, failed)
}

// mxCheckPending fails for every package that does not compile and is not on mxCppPending, and for every package of mxCppPending that now compiles.
func mxCheckPending(t *testing.T, failed map[string]string) {
	t.Helper()
	seen := map[string]bool{}
	for _, src := range slices.Sorted(maps.Keys(failed)) {
		name := strings.TrimSuffix(filepath.Base(src), mxCppSuffix)
		seen[name] = true
		if !slices.Contains(mxCppPending, name) {
			t.Errorf("%s does not compile:\n%s", filepath.Base(src), failed[src])
		}
	}
	for _, name := range mxCppPending {
		if !seen[name] {
			t.Errorf("%s compiles now: remove it from mxCppPending", name)
		}
	}
}

// mxMatrix is a matrix built into a project: its directory ("" when the matrix moved), and its numbers.
type mxMatrix struct {
	dir                     string
	cases, illegal, refused int
}

// mxPrepare decides the matrix by Canon's check (the allowlist, the pinned positions), writes the project of the positions it keeps and builds it with no finding, emitting go when goGen, else cpp. Its dir is "" when the allowlist or the pins moved.
func mxPrepare(t *testing.T, goGen bool) mxMatrix {
	t.Helper()
	cases, illegal := mxCases(goGen)
	kept, refused := mxRefusals(t, goGen, cases)
	mxCheckAllowlist(t, refused)
	mxCheckPinned(t, cases, kept)
	m := mxMatrix{cases: len(cases), illegal: illegal, refused: len(refused)}
	if t.Failed() {
		return m
	}
	var files []mxFile
	for _, c := range cases {
		files = append(files, mxCaseFiles(t, c, kept[c.name()])...)
	}
	m.dir = mxWrite(t, goGen, files)
	mxBuild(t, m.dir)
	return m
}

// mxCaseFiles are the packages of case c, the positions pos it keeps given: one set for the positions that must compile, and, for a case on mxPendingProbe, a second set (tagged mxPendingTag) for the position held back as pending. Both languages write the split, so their package counts agree.
func mxCaseFiles(t *testing.T, c mxCase, pos []mxPos) []mxFile {
	t.Helper()
	held, pending := mxPendingProbe[c.name()]
	if !pending {
		if len(pos) == 0 {
			return nil
		}
		return c.files(pos)
	}
	var rest, probe []mxPos
	for _, p := range pos {
		if p.code == held {
			probe = append(probe, p)
		} else {
			rest = append(rest, p)
		}
	}
	if len(probe) == 0 {
		t.Errorf("%s: position %s is on mxPendingProbe and is not kept", c.name(), held)
	}
	files := c.files(rest)
	c.tag = mxPendingTag
	return append(files, c.files(probe)...)
}

// mxCheckAllowlist fails for every refusal no entry of mxAllowed names, by case, position and the diagnostic code.
func mxCheckAllowlist(t *testing.T, refused []mxRefusal) {
	t.Helper()
	for _, r := range refused {
		if !slices.ContainsFunc(mxAllowed, func(a mxAllowance) bool { return a.code == r.code && a.applies(r.c, r.pos) }) {
			t.Errorf("%s position %s is refused by %s, which no allowance names: %s", r.c.name(), r.pos.code, r.code, r.message)
		}
	}
}

// mxCheckPinned fails when the positions a case keeps are not the ones mxPinned holds, naming the cases, and prints the summary to pin.
func mxCheckPinned(t *testing.T, cases []mxCase, kept map[string][]mxPos) {
	t.Helper()
	var lines []string
	for _, c := range cases {
		var codes strings.Builder
		for _, p := range kept[c.name()] {
			codes.WriteString(p.code)
		}
		lines = append(lines, c.name()+" "+codes.String())
	}
	want := strings.Split(strings.TrimSpace(mxPinned), "\n")
	for i := range max(len(lines), len(want)) {
		if mxAt(lines, i) != mxAt(want, i) {
			t.Errorf("pinned case %d moved: got %q, pinned %q", i, mxAt(lines, i), mxAt(want, i))
		}
	}
	if t.Failed() {
		t.Logf("the summary, to pin if it is right:\n%s", strings.Join(lines, "\n"))
	}
}

// mxAt is line i of lines, "" past the end.
func mxAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
}
