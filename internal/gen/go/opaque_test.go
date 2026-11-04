package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
	"github.com/fantasim/canonlang/internal/types"
)

// opaquePackage is a baked table with two entries and two enums: Tone with a wire name apart from its Canon name, Coded with @codes.
func opaquePackage() *ir.Package {
	status, statuses, _ := lookupTable("Status", "statuses", "open", "taken")
	tone := &ir.Enum{Pkg: lookupsPkg, Name: "Tone", Members: []*ir.EnumMember{{Name: "warning", Wire: "warning"}, {Name: "series_1", Wire: "series-1", Index: 1}}}
	coded := &ir.Enum{Pkg: lookupsPkg, Name: "Coded", Codes: &u8T, Members: []*ir.EnumMember{{Name: "low", Wire: "low", Code: 1}, {Name: "high", Wire: "high", Index: 1, Code: 200}}}
	shape := &ir.Variant{Pkg: lookupsPkg, Name: "Shape", Tag: "kind", Cases: []*ir.Case{{Name: "round", Wire: "round"}, {Name: "flat", Wire: "flat"}}}
	mood := &ir.Dependent{
		Pkg: lookupsPkg, Name: "Mood", Params: 1, DiscParam: 0, Disc: &ir.TypeRef{Kind: types.Enum, Named: tone},
		Branches: []*ir.Branch{{Name: "warning", Members: []int{0}, Type: intT}, {Name: "series", Members: []int{1}, Type: strT}},
	}
	e := goData("demo/opaque", "opaque")
	e.Mode, e.Values = ir.ModeBaked, []string{"statuses"}
	return &ir.Package{Name: lookupsPkg, Dir: "demo/opaque", Types: []ir.Type{status, tone, coded, shape, mood}, Values: []*ir.Value{statuses}, Emits: []*ir.Emit{e}}
}

// CODEGEN.md §5.2, §5.3 (DECISIONS 379): json writes keys and wire names, sql and gob refuse an id, an @codes enum keeps its integer.
func TestIDTypeOpaque(t *testing.T) {
	runData(t, generateData(t, opaquePackage()), "demo/opaque/out/go", "testdata/smoke/opaque_test.go", nil)
}

const opaqueHeader = "package bad\n\nimport opaque \"example.com/data/demo/opaque/out/go\"\n\n"

// buildBad compiles body as package bad beside the generated opaque package and returns the go build outcome.
func buildBad(t *testing.T, body string) (string, error) {
	t.Helper()
	dir := writeModule(t, generateData(t, opaquePackage()), "demo/opaque/out/go", "", nil)
	if err := os.MkdirAll(filepath.Join(dir, "bad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad", "bad.go"), []byte(opaqueHeader+body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	out, err := cxx.Combined(cmd)
	return string(out), err
}

// CODEGEN.md §5.3 (DECISIONS 379): no integer converts to or from an id, and no arithmetic compiles on it.
func TestIDTypeRefusesIntegers(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"conversion to", "var _ = opaque.StatusID(1)"},
		{"constant", "var _ opaque.StatusID = 1"},
		{"conversion from", "var _ = uint8(opaque.StatusIDOpen)"},
		{"arithmetic", "var _ = opaque.StatusIDOpen + 1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if out, err := buildBad(t, c.body); err == nil || !strings.Contains(out, "bad.go") {
				t.Errorf("%s compiled: %v\n%s", c.body, err, out)
			}
		})
	}
}

// CODEGEN.md §5.2, §5.5, §5.6, §2.5 T20 (DECISIONS 402): an enum without @codes, a kind enum and a branch enum are as opaque as an id; Position() is the way to an index.
func TestEnumTypeRefusesIntegers(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"conversion to", "var _ = opaque.Tone(1)"},
		{"constant", "var _ opaque.Tone = 1"},
		{"conversion from", "var _ = uint8(opaque.ToneWarning)"},
		{"arithmetic", "var _ = opaque.ToneWarning + 1"},
		{"order", "var _ = opaque.ToneWarning < opaque.ToneSeries1"},
		{"const", "const _ = opaque.ToneWarning"},
		{"array key", "var _ = [2]int{opaque.ToneWarning: 1}"},
		{"kind conversion", "var _ = opaque.ShapeKind(0)"},
		{"branch conversion", "var _ = opaque.MoodBranch(0)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if out, err := buildBad(t, c.body); err == nil || !strings.Contains(out, "bad.go") {
				t.Errorf("%s compiled: %v\n%s", c.body, err, out)
			}
		})
	}
	t.Run("control", func(t *testing.T) {
		if out, err := buildBad(t, "var _ = opaque.ToneWarning.Position() < opaque.ToneSeries1.Position()"); err != nil {
			t.Errorf("Position comparison did not compile: %v\n%s", err, out)
		}
	})
}
