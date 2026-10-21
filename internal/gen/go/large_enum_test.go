package gogen_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"text/template"
)

// The synthetic enums of TestLargeEnumsCompile (DECISIONS 393): wires and codes that follow neither declaration order nor each other, so every table is searched, never indexed by luck.
const (
	largeMembers    = 12001
	fewMembers      = 10
	manyMembers     = 1000
	genSuffix       = ".gen.go"
	enumMethodExpr  = ` (\w+\.(String|Wire|Known|Code)|Parse\w+|\w+(Members|FromCode))$`
	largeModule     = "example.com/large"
	wireStride      = 7919
	wireModulus     = 99991
	codeTop         = 60000
	codeStep        = 3
	openCodeStride  = 7
	openCodeMod     = 65000
	maxRSSKB        = 1 << 20 // 1 GB of the compiler's peak resident set, in KB
	memberFormat    = "m%05d"
	keyFormat       = "k%05d"
	wireFormat      = "%s%05d"
	constFormat     = "M%05d"
	keyConstFormat  = "K%05d"
	caseFormat      = "c%05d"
	caseConstFormat = "C%05d"
)

// enumMethod matches "file name" of the methods and functions the size guard compares.
var enumMethod = regexp.MustCompile(enumMethodExpr)

// largeRow is the expected data of one member, written into a generated test.
type largeRow struct {
	Const, Name, Wire, Key, KeyConst, Case, CaseConst string
	Code, Pos                                         int
}

// largeWire is member i's wire: zero-padded, so byte order differs from declaration order.
func largeWire(prefix string, i int) string {
	return fmt.Sprintf(wireFormat, prefix, (i*wireStride)%wireModulus)
}

// largeSpecs are the enums of the closed package: one without codes, one with descending codes.
func largeSpecs(n int) (plain, coded enumSpec) {
	plain, coded = enumSpec{Name: "Plain"}, enumSpec{Name: "Coded", Codes: "UInt16"}
	for i := range n {
		member := fmt.Sprintf(memberFormat, i) + "=" + largeWire("W", i)
		plain.Members = append(plain.Members, member)
		coded.Members = append(coded.Members, member)
		coded.CodeValues = append(coded.CodeValues, int64(codeTop-codeStep*i))
	}
	return plain, coded
}

// largeFixture is a closed package (two enums, a table of n entries, baked), a package of a variant of n fieldless cases, so a kind enum of n members (its own package: its make hooks are as many functions), and a types package opening an enum with codes.
func largeFixture(n int) *fixture {
	plain, coded := largeSpecs(n)
	open := enumSpec{Name: "Open", Codes: "UInt16"}
	items := valueSpec{Name: "items", Type: "table Item"}
	shape := variantSpec{Name: "Shape"}
	for i := range n {
		open.Members = append(open.Members, fmt.Sprintf(memberFormat, i)+"="+largeWire("O_", i))
		open.CodeValues = append(open.CodeValues, int64(i*openCodeStride%openCodeMod+1))
		shape.Cases = append(shape.Cases, recordSpec{Name: fmt.Sprintf(caseFormat, i)})
		items.Entries = append(items.Entries, entrySpec{ID: fmt.Sprintf(keyFormat, i), Fields: map[string]json.RawMessage{"label": json.RawMessage(`"x"`)}})
	}
	return &fixture{Module: largeModule, Packages: []*pkgSpec{
		{
			Name: "large.closed", Dir: "large/closed", Enums: []enumSpec{plain, coded},
			Emit:    emitSpec{Out: "out/", Dir: "large/closed", Import: largeModule + "/closed", Package: "closed", Mode: "baked", Values: []string{"items"}},
			Records: []recordSpec{{Name: "Item", Fields: []fieldSpec{{Name: "label", Type: "String"}}}}, Values: []valueSpec{items},
		},
		{
			Name: "large.kinds", Dir: "large/kinds", Variants: []variantSpec{shape},
			Emit: emitSpec{Out: "out/", Dir: "large/kinds", Import: largeModule + "/kinds", Package: "kinds", Mode: "baked"},
		},
		{
			Name: "large.open", Dir: "large/open", Enums: []enumSpec{open},
			Emit: emitSpec{Out: "out/", Dir: "large/open", Import: largeModule + "/open", Package: "open", Mode: "types", Open: []string{"Open"}},
		},
	}}
}

// largeRows are the expected rows at the first, middle and last member.
func largeRows(n int) []largeRow {
	var rows []largeRow
	for _, i := range []int{0, n / 2, n - 1} {
		rows = append(rows, largeRow{
			Const: fmt.Sprintf(constFormat, i), Name: fmt.Sprintf(memberFormat, i), Key: fmt.Sprintf(keyFormat, i), KeyConst: fmt.Sprintf(keyConstFormat, i), Case: fmt.Sprintf(caseFormat, i), CaseConst: fmt.Sprintf(caseConstFormat, i), Pos: i,
			Wire: largeWire("W", i), Code: codeTop - codeStep*i,
		})
	}
	return rows
}

// largeSmoke fills a smoke template of testdata/large: the rows (the open package's wire and code differ) and the member count.
func largeSmoke(t *testing.T, name string, open bool, n int) []byte {
	t.Helper()
	tmpl, err := template.New(filepath.Base(name)).Funcs(template.FuncMap{"q": strconv.Quote}).ParseFiles(name)
	if err != nil {
		t.Fatal(err)
	}
	rows := largeRows(n)
	for i := range rows {
		if open {
			rows[i].Wire, rows[i].Code = largeWire("O_", rows[i].Pos), rows[i].Pos*openCodeStride%openCodeMod+1
		}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		Rows  []largeRow
		Count int
	}{rows, n}); err != nil {
		t.Fatal(err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		t.Fatalf("%s: %v\n%s", name, err, buf.Bytes())
	}
	return src
}

// goCmd runs the installed Go in dir and returns its output and the process, for its resource usage.
func goCmd(t *testing.T, dir string, args ...string) ([]byte, *os.ProcessState) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off", "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out, cmd.ProcessState
}

// largeWorld is the fixture of n members per enum, built and generated.
func largeWorld(t *testing.T, n int) (*world, map[string][]byte) {
	t.Helper()
	return buildWorld(t, largeFixture(n))
}

// buildWorld loads the fixture and generates every package of it.
func buildWorld(t *testing.T, fx *fixture) (*world, map[string][]byte) {
	t.Helper()
	raw, err := json.Marshal(fx)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	w := loadWorld(t, path, listedCells)
	return w, generate(t, w)
}

// writeNoncedModule writes the generated files, each ended by a comment no other run shares (so the Go cache never answers the build), and the smoke tests (by path) into a module in dir.
func writeNoncedModule(t *testing.T, dir, module string, files map[string][]byte, smoke map[string][]byte) {
	t.Helper()
	write := func(name string, content []byte) {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte("module "+module+"\n\ngo "+goVersion(t)+"\n"))
	for _, p := range sortedPaths(files) {
		write(p, append(files[p], []byte("\n// nonce "+dir+"\n")...))
	}
	for _, name := range slices.Sorted(maps.Keys(smoke)) {
		write(name, smoke[name])
	}
}

// CODEGEN.md §5.2, §5.3, DECISIONS 393: the enum methods (String, Wire, Parse<E>, <E>Members, Code, <E>FromCode, Known) do not grow with the member count, so a package of more than 12,000 members per enum compiles in a bounded resident set, and those methods answer as the switches did, at the first, middle and last member and outside the enum.
func TestLargeEnumsCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	_, files := largeWorld(t, largeMembers)
	mod := t.TempDir()
	writeNoncedModule(t, mod, largeModule, files, map[string][]byte{
		"closed/smoke_test.go": largeSmoke(t, "testdata/large/closed_smoke.tmpl", false, largeMembers),
		"kinds/smoke_test.go":  largeSmoke(t, "testdata/large/kinds_smoke.tmpl", false, largeMembers),
		"open/smoke_test.go":   largeSmoke(t, "testdata/large/open_smoke.tmpl", true, largeMembers),
	})
	_, state := goCmd(t, mod, "build", "./...")
	checkRSS(t, state)
	goCmd(t, mod, "vet", "./...")
	goCmd(t, mod, "test", "-count=1", "./...")
}

// functionLines is the number of lines of every function and method of the generated files, by file and name.
func functionLines(t *testing.T, files map[string][]byte) map[string]int {
	t.Helper()
	lines := map[string]int{}
	for _, path := range sortedPaths(files) {
		if !strings.HasSuffix(path, genSuffix) {
			continue
		}
		src := files[path]
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				lines[path+" "+funcName(fn)] = fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1
			}
		}
	}
	return lines
}

// funcName is Recv.Name for a method, Name for a function.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil {
		return fn.Name.Name
	}
	return types.ExprString(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

// CODEGEN.md §5.2, §5.3, DECISIONS 393: an enum method (String, Wire, Parse<E>, <E>Members, Code, <E>FromCode, Known) has the same size at 10 members and at 1000.
func TestEnumFunctionsDoNotGrow(t *testing.T) {
	_, few := largeWorld(t, fewMembers)
	_, many := largeWorld(t, manyMembers)
	small, large := functionLines(t, few), functionLines(t, many)
	for _, name := range []string{"closed/closed.gen.go Plain.String", "closed/closed.gen.go ParseCoded", "closed/closed.gen.go ItemID.String", "kinds/kinds.gen.go ShapeKind.Wire", "kinds/kinds.gen.go ParseShapeKind", "open/open.gen.go Open.Known", "open/open.gen.go OpenFromCode"} {
		if small[name] == 0 {
			t.Errorf("no function %s", name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(small)) {
		if enumMethod.MatchString(name) && large[name] != small[name] {
			t.Errorf("%s has %d lines at %d members and %d at %d", name, small[name], fewMembers, large[name], manyMembers)
		}
	}
}

// checkRSS fails when the compile of the generated packages, the largest process the build waited for, peaked above maxRSSKB (Linux reports KB).
func checkRSS(t *testing.T, state *os.ProcessState) {
	t.Helper()
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || runtime.GOOS != "linux" {
		t.Skip("no peak resident set to read")
	}
	t.Logf("go build peak resident set: %d KB", usage.Maxrss)
	if usage.Maxrss > maxRSSKB {
		t.Errorf("go build peaked at %d KB, over %d KB", usage.Maxrss, maxRSSKB)
	}
}
