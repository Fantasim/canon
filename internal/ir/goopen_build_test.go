package ir_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

const e2eGoOpenProject = "testdata/e2e/goopen.txtar"

// TestGoOpenEnumsBuild is DECISIONS 339 and 340 end to end: a project whose go types emit opens enums and whose @text fns return a record, a map, a list, a keyed list and a maybe-file builds with no finding, a result no Go reader holds getting no decoder and no refusal; the generated decoders read the rendered files back, keep a member the enum lacks in a field, a list element, a map key and another package's reader, and still refuse what an opened enum cannot hold (gen/go/b/open_test.go, gen/go/c/open_test.go).
func TestGoOpenEnumsBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	writeProject(t, dir, e2eGoOpenProject)
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if out := renderFindings(t, res.Findings); res.Summary.Errors > 0 {
		t.Fatalf("build findings:\n%s", out)
	}
	gen := filepath.Join(dir, e2eGenRoot)
	writeGoMod(t, gen, e2eGoModule)
	runIn(t, gen, exec.Command("go", "vet", "./..."))
	runIn(t, gen, exec.Command("go", "test", "-race", "-count=1", "./..."))
	checkTextDecoders(t, filepath.Join(gen, "go", "b", "b.gen.go"), []string{"DecodeHolesFile", "DecodeGapsFile", "DecodeGridsFile", "DecodeNothingFile", "DecodeHiddenFile"},
		[]string{"DecodeBonusesFile", "DecodeDstsFile", "DecodeGradedFile", "DecodeSlotsFile", "DecodeWaitsFile"})
	checkTextDecoders(t, filepath.Join(gen, "go", "c", "c.gen.go"), []string{"DecodePricedFile", "DecodeLinksFile"}, []string{"DecodeUsesFile", "DecodeBonusesFile", "DecodeFlatsFile"})
}

// checkTextDecoders is DECISIONS 340: a @text fn whose result no Go reader holds (optional elements or map values, a table, a Never, a local record, another package's record with a computed default or without a make hook) gets no Decode<Fn>File and no finding, beside the ones that do.
func checkTextDecoders(t *testing.T, file string, absent, present []string) {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range absent {
		if strings.Contains(string(src), "func "+name+"(") {
			t.Errorf("%s: %s is written", file, name)
		}
	}
	for _, name := range present {
		if !strings.Contains(string(src), "func "+name+"(") {
			t.Errorf("%s: no %s", file, name)
		}
	}
}
