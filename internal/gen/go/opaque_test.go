package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// opaquePackage is a baked table with two entries and two enums: Tone with a wire name apart from its Canon name, Coded with @codes.
func opaquePackage() *ir.Package {
	status, statuses, _ := lookupTable("Status", "statuses", "open", "taken")
	tone := &ir.Enum{Pkg: lookupsPkg, Name: "Tone", Members: []*ir.EnumMember{{Name: "warning", Wire: "warning"}, {Name: "series_1", Wire: "series-1", Index: 1}}}
	coded := &ir.Enum{Pkg: lookupsPkg, Name: "Coded", Codes: &u8T, Members: []*ir.EnumMember{{Name: "low", Wire: "low", Code: 1}, {Name: "high", Wire: "high", Index: 1, Code: 200}}}
	e := goData("demo/opaque", "opaque")
	e.Mode, e.Values = ir.ModeBaked, []string{"statuses"}
	return &ir.Package{Name: lookupsPkg, Dir: "demo/opaque", Types: []ir.Type{status, tone, coded}, Values: []*ir.Value{statuses}, Emits: []*ir.Emit{e}}
}

// CODEGEN.md §5.2, §5.3 (DECISIONS 379): json writes keys and wire names, sql and gob refuse an id, an @codes enum keeps its integer.
func TestIDTypeOpaque(t *testing.T) {
	runData(t, generateData(t, opaquePackage()), "demo/opaque/out/go", "testdata/smoke/opaque_test.go", nil)
}

// CODEGEN.md §5.3 (DECISIONS 379): no integer converts to or from an id, and no arithmetic compiles on it.
func TestIDTypeRefusesIntegers(t *testing.T) {
	const header = "package bad\n\nimport opaque \"example.com/data/demo/opaque/out/go\"\n\n"
	for _, c := range []struct{ name, body string }{
		{"conversion to", "var _ = opaque.StatusID(1)"},
		{"constant", "var _ opaque.StatusID = 1"},
		{"conversion from", "var _ = uint8(opaque.StatusIDOpen)"},
		{"arithmetic", "var _ = opaque.StatusIDOpen + 1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeModule(t, generateData(t, opaquePackage()), "demo/opaque/out/go", "", nil)
			if err := os.MkdirAll(filepath.Join(dir, "bad"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bad", "bad.go"), []byte(header+c.body+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "build", "./...")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "bad.go") {
				t.Errorf("%s compiled: %v\n%s", c.body, err, out)
			}
		})
	}
}
