package ir_test

import (
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

const (
	mxGoSuffix   = ".gen.go"
	mxGoPkgMark  = "# "
	mxGoExitText = "exit status"
	mxGoList     = "list"
)

// mxGoPackages is the number of Go packages the Go matrix vets, the libraries' included, as `go list ./...` names them: the C++ matrix's packages (mxPackages), less the runtime package each one holds a copy of.
const mxGoPackages = mxPackages

// mxGoRuntime is the import path suffix of the runtime package each generated package holds.
const mxGoRuntime = "/rt"

// mxGoListed are the packages `go list ./...` names in gen, the runtime copies left out, in path order.
func mxGoListed(t *testing.T, gen string) []string {
	t.Helper()
	cmd := exec.Command(goRunBinary, mxGoList, goRunAll)
	cmd.Dir = gen
	out, err := cxx.Output(cmd)
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []string
	for _, p := range strings.Fields(string(out)) {
		if !strings.HasSuffix(p, mxGoRuntime) {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs
}

// mxGoErrors is what `go vet ./...` says in gen, by package: the first lines of each package that does not compile or vet.
func mxGoErrors(t *testing.T, gen string) map[string]string {
	t.Helper()
	cmd := exec.Command(goRunBinary, goRunVet, goRunAll)
	cmd.Dir = gen
	out, err := cxx.Combined(cmd)
	if err == nil {
		return nil
	}
	failed := map[string]string{}
	var pkg string
	for _, l := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(l, mxGoPkgMark):
			pkg = strings.TrimPrefix(l, mxGoPkgMark)
		case pkg != "" && strings.TrimSpace(l) != "" && !strings.HasPrefix(l, mxGoExitText) && strings.Count(failed[pkg], "\n") < mxErrorLines:
			failed[pkg] += "    " + l + "\n"
		}
	}
	if len(failed) == 0 {
		t.Fatalf("go vet failed with no package named: %v\n%s", err, out)
	}
	return failed
}

// TestGoRefMatrix is CODEGEN.md §5.6, §5.8 and DECISIONS 387, 456 vetted, once for every combination Canon accepts that TestCppRefMatrix compiles (the same leaves, depths, positions and modes, decided by the same pinned check verdicts and allowlist, mxPrepare): with go vet over the Go module of the build. A combination that does not compile is a bug of gen/go's.
func TestGoRefMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	m := mxPrepare(t, true)
	if m.dir == "" {
		return // nothing is compiled of a matrix that moved
	}
	gen := filepath.Join(m.dir, e2eGenRoot)
	writeGoMod(t, gen, e2eGoModule)
	pkgs := mxGoListed(t, gen)
	if len(pkgs) != mxGoPackages {
		t.Errorf("%d packages listed by go list (the libraries included), %d pinned", len(pkgs), mxGoPackages)
	}
	failed := mxGoErrors(t, gen)
	t.Logf("%d cases (%d illegal: an own table's ids are an enum in a baked package only), %d positions refused by the allowlist, %d packages vetted, %d fail", m.cases, m.illegal, m.refused, len(pkgs), len(failed))
	for _, pkg := range slices.Sorted(maps.Keys(failed)) {
		t.Errorf("%s does not compile:\n%s", pkg, failed[pkg])
	}
}
