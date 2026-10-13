package ir_test

import (
	"context"
	"encoding/json"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	refRunProject  = "testdata/e2e/refrun/project.txtar"
	refRunProbes   = "testdata/e2e/refrun"
	refRunData     = "app/out/app/book.json"
	refRunLoaded   = "probe/out/book.json"
	refRunDataExt  = ".data"
	refRunLoadPkg  = "probe"
	refRunLoadFile = "book.json"
	// refRunLoader loads book.json as lib's Book and writes the value `load` gives as a data file.
	refRunLoader  = "package probe\n\nimport lib { Book }\n\n/// The book under test.\nlet book: Book = load(\"book.json\")\n\nemit json { out: \"out/\" }\n"
	refRunOK      = "ok "
	refRunNoBr    = "error no branch"
	refRunOther   = "error other"
	refRunCppMain = "probe.cpp"
	refRunCppBin  = "probe"
	refRunTSOut   = "js"
	refRunTSMain  = "probe.mjs"
	refRunTSPkg   = "package.json"
	// refRunTSModule makes node read the compiled .js files as ES modules.
	refRunTSModule = "{\"type\": \"module\"}\n"
	// refRunBase is the verdict on refRunEntry() with nothing replaced.
	refRunBase = "kind=count payload=num:3 more=[num:1,num:2] ev=b byEv=num:7 num=10 byNum=text:t flag=yes byFlag=true:4 alias=second byAlias=text:s"
)

// refRunFields are an Entry's fields with a valid value each: a table key (kind), an enum key whose @codes are out of member order (ev), an Int key (num), a Bool discriminant (flag) and a ref to a ref (alias).
var refRunFields = [][2]string{
	{"kind", `"count"`}, {"payload", "3"}, {"more", "[1, 2]"}, {"ev", `"b"`}, {"byEv", "7"}, {"num", "10"},
	{"byNum", `"t"`}, {"flag", `"yes"`}, {"byFlag", "4"}, {"alias", `"second"`}, {"byAlias", `"s"`},
}

// refRunEntry is an Entry's JSON: refRunFields, each pair of over replacing a field's value, or dropping the field for "".
func refRunEntry(over ...string) string {
	var parts []string
	for _, f := range refRunFields {
		v := f[1]
		if i := slices.Index(over, f[0]); i >= 0 && i%2 == 0 && i+1 < len(over) {
			v = over[i+1]
		}
		if v != "" {
			parts = append(parts, `"`+f[0]+`": `+v)
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// refRunBook is a Book of entries.
func refRunBook(entries ...string) string {
	return `{"entries": [` + strings.Join(entries, ", ") + "]}"
}

// refRunCase is one book file, the verdict every reader must give on it, and whether `load` accepts it.
type refRunCase struct {
	name, json, want string
	loads            bool
}

// refRunCases are DECISIONS 350's outcomes for each kind of key: a key the table holds decodes in its branch (a list sharing it, a none or an empty list where the path meets none or a Never arm); a key the table lacks, a value where the path meets none and a value in a Never arm are `no branch for this value`; a value of the other branch's type, or an enum key no member has, is refused otherwise.
var refRunCases = []refRunCase{
	{"good", refRunBook(refRunEntry(),
		refRunEntry("kind", `"blank"`, "payload", "", "more", "[]", "ev", `"a"`, "byEv", `"x"`, "num", "2", "byNum", "5", "flag", `"no"`, "byFlag", `"f"`, "alias", `"first"`, "byAlias", "9"),
		refRunEntry("kind", `"old"`, "payload", "", "more", "[]", "ev", `"c"`, "byEv", "")),
		refRunOK + refRunBase +
			"; kind=blank payload=none more=[] ev=a byEv=text:x num=2 byNum=num:5 flag=no byFlag=false:f alias=first byAlias=num:9" +
			"; kind=old payload=none more=[] ev=c byEv=none num=10 byNum=text:t flag=yes byFlag=true:4 alias=second byAlias=text:s", true},
	{"unknown_key", refRunBook(refRunEntry("kind", `"ghost"`)), refRunNoBr, false},
	{"none_on_path", refRunBook(refRunEntry("kind", `"blank"`, "payload", "1", "more", "[]")), refRunNoBr, false},
	{"never_arm", refRunBook(refRunEntry("kind", `"old"`, "payload", "1", "more", "[]")), refRunNoBr, false},
	{"never_in_list", refRunBook(refRunEntry("kind", `"old"`, "payload", "", "more", "[2]")), refRunNoBr, false},
	{"enum_key_never", refRunBook(refRunEntry("ev", `"c"`, "byEv", "1")), refRunNoBr, false},
	{"int_key_missing", refRunBook(refRunEntry("num", "99")), refRunNoBr, false},
	{"enum_key_unknown", refRunBook(refRunEntry("ev", `"zz"`)), refRunOther, false},
	{"bool_branch_type", refRunBook(refRunEntry("flag", `"no"`)), refRunOther, false},
	{"chain_branch_type", refRunBook(refRunEntry("alias", `"first"`)), refRunOther, false},
	{"wrong_branch_type", refRunBook(refRunEntry("payload", `"x"`)), refRunOther, false},
}

// TestRefDependentRoundTrip is DECISIONS 350 and CODEGEN.md §5.6 end to end: on the same JSON, `load` and every reader generated for a dependent value decided through a ref agree, whatever the key (a table id, an enum with @codes out of member order, an Int, a ref to a ref) or the discriminant (an enum, a Bool). lib's types-mode decoders read the JSON; app's data loaders (reading lib's Entry with app's own readers and tables) read the data file Canon writes of the value `load` gave; both decode it as `load` did, in Go, C++ and TypeScript, and refuse what `load` refuses, a key their table lacks, a none on the path and a Never arm with `no branch for this value`.
func TestRefDependentRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := t.TempDir()
	gen := refRunBuild(t, dir)
	schema := refRunSchema(t, dir)
	cases := filepath.Join(dir, "cases")
	if err := os.MkdirAll(cases, e2eDirMode); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, c := range refRunCases {
		files = append(files, refRunFile(t, dir, cases, schema, c))
	}
	t.Run("go", func(t *testing.T) { refRunCompare(t, refRunGo(t, gen, files)) })
	t.Run("cpp", func(t *testing.T) { refRunCompare(t, refRunCpp(t, gen, files)) })
	t.Run("ts", func(t *testing.T) { refRunCompare(t, refRunTS(t, gen, files)) })
}

// refRunBuild builds the project of refRunProject in dir with no finding and returns its gen directory.
func refRunBuild(t *testing.T, dir string) string {
	t.Helper()
	writeProject(t, dir, refRunProject)
	if err := os.WriteFile(filepath.Join(dir, e2eProjFile), []byte(e2eFeatures), e2eFileMode); err != nil {
		t.Fatal(err)
	}
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
	return filepath.Join(dir, e2eGenRoot)
}

// refRunSchema is the $schema of app's data file of a Book.
func refRunSchema(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(refRunData)))
	if err != nil {
		t.Fatal(err)
	}
	var head struct {
		Schema string `json:"$schema"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.Schema == "" {
		t.Fatalf("no $schema in %s: %v", refRunData, err)
	}
	return head.Schema
}

// refRunFile writes c's book under cases and its .data twin, the data file Canon writes of the value `load` gives (WIRE.md §8.1), or, where `load` refuses the book, the book wrapped as one; it returns the book's path.
func refRunFile(t *testing.T, dir, cases, schema string, c refRunCase) string {
	t.Helper()
	data, loads := refRunLoads(t, dir, c)
	if loads != c.loads {
		t.Errorf("%s: load accepts it: %v, want %v", c.name, loads, c.loads)
	}
	if !loads {
		data = []byte(`{"$schema": "` + schema + `", "value": ` + c.json + "}\n")
	}
	path := filepath.Join(cases, c.name+e2eJSONSuffix)
	if err := os.WriteFile(path, []byte(c.json+"\n"), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+refRunDataExt, data, e2eFileMode); err != nil {
		t.Fatal(err)
	}
	return path
}

// refRunLoads builds a package loading c's book as lib's Book (WIRE.md §5) and returns the data file its json emit writes, and whether the build accepts the book.
func refRunLoads(t *testing.T, dir string, c refRunCase) ([]byte, bool) {
	t.Helper()
	pkg := filepath.Join(dir, refRunLoadPkg)
	if err := os.MkdirAll(pkg, e2eDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, refRunLoadPkg+canonExt), []byte(refRunLoader), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, refRunLoadFile), []byte(c.json), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{refRunLoadPkg}})
	if err != nil {
		t.Fatal(err) // a Go error is a broken build, never `load` refusing the book
	}
	if res.Summary.Errors > 0 {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(refRunLoaded)))
	if err != nil {
		t.Fatal(err)
	}
	return data, true
}

// refRunGo runs the Go probe, which decodes every book with lib's DecodeBook and app's LoadBook.
func refRunGo(t *testing.T, gen string, files []string) []string {
	t.Helper()
	mod := e2eGoModPrefix + e2eGoModule + e2eGoModGo + strings.TrimPrefix(version.Lang(runtime.Version()), "go") + "\n"
	if err := os.WriteFile(filepath.Join(gen, e2eGoModFile), []byte(mod), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	refRunCopy(t, "probe_go.txt", filepath.Join(gen, "probe", "main.go"))
	return refRunOutput(t, gen, "", exec.Command("go", append([]string{"run", "./probe"}, files...)...))
}

// refRunCpp compiles and runs the C++ probe with each compiler found, against lib's and app's .gen.cpp; without one it skips, or fails where the C++ toolchain is required.
func refRunCpp(t *testing.T, gen string, files []string) []string {
	t.Helper()
	compilers, include := cxx.Toolchain(t)
	refRunCopy(t, "probe_cpp.txt", filepath.Join(gen, refRunCppMain))
	var out []string
	for _, cc := range compilers {
		bin := filepath.Join(gen, refRunCppBin+"-"+filepath.Base(cc))
		args := append(slices.Clone(cxx.Flags), "-I", include, "-I", gen, "-o", bin, refRunCppMain,
			filepath.Join("libc", "lib.gen.cpp"), filepath.Join("appc", "app.gen.cpp"))
		runIn(t, gen, exec.Command(cc, args...))
		out = append(out, refRunOutput(t, gen, filepath.Base(cc), exec.Command(bin, files...))...)
	}
	return out
}

// refRunTS compiles lib.ts and app.ts to JavaScript with each compiler found and runs the TypeScript probe on them with node; without one it skips, or fails where TypeScript is required.
func refRunTS(t *testing.T, gen string, files []string) []string {
	t.Helper()
	tc := tsc.Find(t)
	var out []string
	for _, c := range tc.Compilers {
		js := filepath.Join(gen, refRunTSOut+"-"+c.Name)
		args := append([]string{c.Script, "--outDir", js}, tsc.Args(tc.Typings, "lib.ts", "app.ts")...)
		runIn(t, gen, exec.Command(tc.Node, args...))
		refRunCopy(t, "probe_ts.txt", filepath.Join(js, refRunTSMain))
		if err := os.WriteFile(filepath.Join(js, refRunTSPkg), []byte(refRunTSModule), e2eFileMode); err != nil {
			t.Fatal(err)
		}
		out = append(out, refRunOutput(t, gen, c.Name, exec.Command(tc.Node, append([]string{filepath.Join(js, refRunTSMain)}, files...)...))...)
	}
	return out
}

// refRunCopy copies the probe source name to path.
func refRunCopy(t *testing.T, name, path string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(refRunProbes, name))
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

// refRunOutput runs cmd in dir and returns its output lines, each reader tagged with the toolchain that built it.
func refRunOutput(t *testing.T, dir, tag string, cmd *exec.Cmd) []string {
	t.Helper()
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	for i, l := range lines {
		if reader, verdict, ok := strings.Cut(l, " "); ok && tag != "" {
			lines[i] = reader + "/" + tag + " " + verdict
		}
	}
	return lines
}

// refRunCompare checks every probe line, `<reader> <verdict>`, in case order per reader, against the case's verdict, and that each reader gave one per case.
func refRunCompare(t *testing.T, lines []string) {
	t.Helper()
	readers := map[string]int{}
	for _, line := range lines {
		t.Log(line)
		reader, got, _ := strings.Cut(line, " ")
		i := readers[reader]
		readers[reader]++
		if i >= len(refRunCases) {
			t.Errorf("%s: more verdicts than cases: %s", reader, line)
			continue
		}
		if c := refRunCases[i]; got != c.want {
			t.Errorf("%s on %s: %s, want %s", reader, c.name, got, c.want)
		}
	}
	for reader, n := range readers { //canon:unordered each reader is checked alone
		if n != len(refRunCases) {
			t.Errorf("%s gave %d verdicts for %d cases", reader, n, len(refRunCases))
		}
	}
	if len(readers) == 0 {
		t.Error("no verdict")
	}
}
