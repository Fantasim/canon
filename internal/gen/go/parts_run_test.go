package gogen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
)

const (
	runRows     = 60
	runFields   = 30
	runLimit    = 100
	dumpEnv     = "PARTS_OUT"
	dumpName    = "dump.txt"
	partsCaller = "_0("
)

// wideSample is the good wire value of field i.
func wideSample(i int) string {
	wire := wideShapes[i%shapeCount].wire
	if strings.Contains(wire, "%") {
		return fmt.Sprintf(wire, i)
	}
	return wire
}

// wideSmoke is a test of package wide that decodes a good document, the document without each field, and the document with each field `{}`, and writes what every decoder answered (value or error text) to the file $PARTS_OUT.
func wideSmoke(n int) []byte {
	var fields, getters []string
	for i := range n {
		fields = append(fields, fmt.Sprintf("{%q, %q}", fmt.Sprintf(wideField, i), wideSample(i)))
		getters = append(getters, fmt.Sprintf("fmt.Sprint(w.F%02d())", i))
	}
	return fmt.Appendf(nil, `package wide_test

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	wide "example.com/parts/wide"
)

var fields = [][2]string{%s}

// document holds every field's good value but skip's, and the bad ones as {}.
func document(skip string, bad ...string) string {
	var parts []string
	for _, f := range fields {
		switch {
		case f[0] == skip:
		case slices.Contains(bad, f[0]):
			parts = append(parts, fmt.Sprintf("%%q: {}", f[0]))
		default:
			parts = append(parts, fmt.Sprintf("%%q: %%s", f[0], f[1]))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func TestDump(t *testing.T) {
	var b strings.Builder
	run := func(label, text string) {
		w, err := wide.DecodeWide([]byte(text))
		if err != nil {
			fmt.Fprintf(&b, "%%s: %%v\n", label, err)
			return
		}
		fmt.Fprintf(&b, "%%s: %%v\n", label, []string{%s})
	}
	run("good", document(""))
	for _, f := range fields {
		run("missing "+f[0], document(f[0]))
		run("bad "+f[0], document("", f[0]))
	}
	run("two", document("", "f20", "f03"))
	if err := os.WriteFile(os.Getenv("PARTS_OUT"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
`, strings.Join(fields, ", "), strings.Join(getters, ", "))
}

// rowsSmoke is a test of package rows that writes every row of the table and of the keyed list, the lookups and the record value to the file $PARTS_OUT.
const rowsSmoke = `package rows_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	rows "example.com/parts/rows"
)

func TestDump(t *testing.T) {
	var b strings.Builder
	items, named, home := rows.GetItems(), rows.GetNamed(), rows.GetHome()
	fmt.Fprintf(&b, "items %d named %d\n", items.Len(), named.Len())
	for i := range items.Len() {
		r := items.At(i)
		fmt.Fprintf(&b, "%s %s %d %s\n", r.ID(), r.Label(), r.N(), r.Tone())
		if got, ok := items.Find(r.ID().String()); !ok || got != r {
			fmt.Fprintf(&b, "Find(%s) = %v %v\n", r.ID(), got, ok)
		}
	}
	for i := range named.Len() {
		r := named.At(i)
		fmt.Fprintf(&b, "%s %d\n", r.Label(), r.N())
		if got, ok := named.Find(r.Label()); !ok || got != r {
			fmt.Fprintf(&b, "Find(%s) = %v %v\n", r.Label(), got, ok)
		}
	}
	fmt.Fprintf(&b, "home %s %d %s\n", home.Label(), home.N(), home.Tone())
	if err := os.WriteFile(os.Getenv("PARTS_OUT"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
`

// dumpOf generates the fixture with gen, builds and vets it in a temporary module, and returns what its smoke tests wrote.
func dumpOf(t *testing.T, gen generator, rows, wide int) string {
	t.Helper()
	files := generateWith(t, gen, loadFixture(t, partsFixture(rows, wide)))
	mod, out := t.TempDir(), t.TempDir()
	writeNoncedModule(t, mod, partsModule, files, map[string][]byte{
		"rows/smoke_test.go": []byte(rowsSmoke), "wide/smoke_test.go": wideSmoke(wide),
	})
	goCmd(t, mod, "vet", "./...")
	var dump strings.Builder
	for _, pkg := range []string{"rows", "wide"} {
		t.Setenv(dumpEnv, filepath.Join(out, pkg+"-"+dumpName))
		goCmd(t, mod, "test", "-count=1", "./"+pkg)
		b, err := os.ReadFile(filepath.Join(out, pkg+"-"+dumpName))
		if err != nil {
			t.Fatal(err)
		}
		dump.Write(b)
	}
	return dump.String()
}

// CODEGEN.md §2.11, DECISIONS 435: a cut build and a cut decoder run as the whole did: the same values (60 rows in a table and a keyed list, a record value) and the same first failure (each field missing, each field bad, two bad), the program vetted; the cut generation really has parts.
func TestCutRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	whole := dumpOf(t, gogen.Generate, runRows, runFields)
	cut := dumpOf(t, cutAt(runLimit), runRows, runFields)
	if whole != cut {
		t.Errorf("the cut program answers differently:\n%s\nvs\n%s", whole, cut)
	}
	for _, want := range []string{"good: [", "items 60 named 60", "missing f00: ", "bad f29: ", "two: "} {
		if !strings.Contains(whole, want) {
			t.Errorf("the dump has no %q", want)
		}
	}
	if strings.Contains(whole, "Find(") {
		t.Errorf("a lookup failed:\n%s", whole)
	}
	files := generateWith(t, cutAt(runLimit), loadFixture(t, partsFixture(runRows, runFields)))
	requireParts(t, files, rowsFile, buildFn)
	requireParts(t, files, wideFile, decodeFn)
	if !strings.Contains(string(files[wideFile]), decodeFn+partsCaller) {
		t.Errorf("%s does not call its first part", decodeFn)
	}
}
