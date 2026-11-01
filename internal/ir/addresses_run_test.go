package ir_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	addressesProject = "testdata/e2e/addresses/project.txtar"
	addressesDir     = "testdata/e2e/addresses"
	addressesCppSrc  = "probe.cpp.txt"
	addressesGoSrc   = "probe_go.txt"
	addressesTSSrc   = "probe_ts.txt"
	addressesData    = "da/out/items.json"
	addressesReload  = "reload"
	addressesGoProbe = "probe"
	addressesGoFile  = "probe_test.go"
	addressesTSOut   = "js"
	addressesTSMain  = "probe.mjs"
	addressesTSPkg   = "package.json"
	addressesTSType  = "{\"type\": \"module\"}\n"
	addressesOK      = " ok"
	addressesVerbose = "-v"
	// The checks each probe holds, pinned here so that deleting one from its source fails the test.
	addressesCppChecks = 29
	addressesGoChecks  = 18
	addressesTSChecks  = 7
)

// addressesCheck is the name a probe gives each of its checks: Check("name", …) in C++, check("name", …) in TypeScript, check(t, "name", …) in Go.
var addressesCheck = regexp.MustCompile(`[Cc]heck\((?:t, )?"([^"]+)"`)

// addressesLine is a line a probe prints for a check that holds.
var addressesLine = regexp.MustCompile(`^[a-z0-9-]+ ok$`)

// addressesWant is the output a probe prints when every check holds: `<name> ok` for each check of its source, in order, the number of them being the one pinned for that probe.
func addressesWant(t *testing.T, name string, checks int) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(addressesDir, name))
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for _, m := range addressesCheck.FindAllStringSubmatch(string(src), -1) {
		want.WriteString(m[1] + addressesOK + "\n")
	}
	if n := strings.Count(want.String(), "\n"); n != checks {
		t.Fatalf("%s names %d checks, want the %d pinned here: a check was deleted or added", name, n, checks)
	}
	return want.String()
}

// addressesCopy copies the probe source name to path.
func addressesCopy(t *testing.T, name, path string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(addressesDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), e2eDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, src, e2eFileMode); err != nil {
		t.Fatal(err)
	}
}

// TestStableAddressesCpp is DECISIONS 401 and CODEGEN.md §5.9 (Addresses) compiled and run in C++: in baked data `Find("open")` is `&At(0)`, a resolved getter and a resolved element give the address `Find` gives on their target, and a repeated accessor gives the same address; a types-mode holder's `Find(1)` is `&At(0)`, and after `Holder m = std::move(*h)` (and a moved KeyedList or FlatMap) the old addresses are the new ones, while a copy has rows of its own; in a data loader a row pointer held across `Store::Reload` still reads its old value in the snapshot held. Built with every compiler, with exceptions and without.
func TestStableAddressesCpp(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, addressesProject)
	addressesCopy(t, addressesCppSrc, filepath.Join(dir, cppRunProbe))
	if err := os.MkdirAll(filepath.Join(dir, cppRunCases, addressesReload), e2eDirMode); err != nil {
		t.Fatal(err)
	}
	want := addressesWant(t, addressesCppSrc, addressesCppChecks)
	outs := cppRunOutputs(t, dir, nil)
	if len(outs) == 0 {
		t.Fatal("no build ran")
	}
	for _, got := range outs {
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	}
}

// TestStableAddressesGo is DECISIONS 401 and CODEGEN.md §5.9 (Addresses) built and run in Go: in baked data `At(0)` is `Find(k)`, `Get(id)`, the first of `All()`, a copied KeyedList gives the same pointer, a resolved getter and a resolved element are `Find` on their target; a decoded holder's keyed list gives one pointer per row however it is reached; a pointer into a snapshot survives `Store.Reload` and the held snapshot still gives it.
func TestStableAddressesGo(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, addressesProject)
	gen := filepath.Join(dir, e2eGenRoot)
	writeGoMod(t, gen, e2eGoModule)
	addressesCopy(t, addressesGoSrc, filepath.Join(gen, addressesGoProbe, addressesGoFile))
	runIn(t, gen, exec.Command(goRunBinary, goRunVet, goRunAll))
	var got strings.Builder
	for _, line := range refRunOutput(t, gen, "", exec.Command(goRunBinary, goRunTest, goRunNoCache, addressesVerbose, goRunAll)) {
		if addressesLine.MatchString(line) {
			got.WriteString(line + "\n")
		}
	}
	if want := addressesWant(t, addressesGoSrc, addressesGoChecks); got.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", got.String(), want)
	}
}

// TestStableAddressesTS is DECISIONS 401 and CODEGEN.md §5.9 (Addresses) compiled and run in TypeScript: for one row `find(k) === at(i) === all[i]`, in a baked table, a baked keyed list and a decoded data file, and a second lookup gives the same row.
func TestStableAddressesTS(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, addressesProject)
	gen := filepath.Join(dir, e2eGenRoot)
	tc := tsc.Find(t)
	want := addressesWant(t, addressesTSSrc, addressesTSChecks)
	for _, c := range tc.Compilers {
		js := filepath.Join(gen, addressesTSOut+"-"+c.Name)
		args := append([]string{c.Script, "--outDir", js}, tsc.Args(tc.Typings, "bk.ts", "ty.ts", "da.ts")...)
		runIn(t, gen, exec.Command(tc.Node, args...))
		addressesCopy(t, addressesTSSrc, filepath.Join(js, addressesTSMain))
		if err := os.WriteFile(filepath.Join(js, addressesTSPkg), []byte(addressesTSType), e2eFileMode); err != nil {
			t.Fatal(err)
		}
		run := exec.Command(tc.Node, filepath.Join(js, addressesTSMain), filepath.Join(dir, filepath.FromSlash(addressesData)))
		if got := strings.Join(refRunOutput(t, gen, "", run), "\n") + "\n"; got != want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", c.Name, got, want)
		}
	}
}
