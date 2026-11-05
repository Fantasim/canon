package canon_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// rowsSrc is package a's table rows with the entries given, emitted as JSON under @out/a/.
func rowsSrc(entries string) []byte {
	return []byte("/// A.\npackage a\n\n/// A row.\nrecord Row {\n  /// Its value.\n  v: Int = 0\n}\n\n" +
		"/// Rows.\nlet rows: table Row = {\n" + entries + "}\n\nemit json { out: \"@out/a/\" }\n")
}

// rowsLayer is a layer dev amending rows' entry one.
var rowsLayer = []byte("package a\nlayer dev\n\namend rows {\n  one.v: 9\n}\n")

// rowsOut is where rowsSrc's build writes.
const rowsOut = "/law/out/a/rows.json"

// rowsProject is buildTestProject with package a of rowsSrc(entries), and rowsLayer when layered.
func rowsProject(entries string, layered bool) *memFS {
	files := map[string][]byte{"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": rowsSrc(entries)}
	if layered {
		files["/law/a/dev.layer.canon"] = rowsLayer
	}
	return newMemFS(files)
}

// DECISIONS 469, API.md B1a, F11: a build with no layer active evaluates every declared layer; one
// the base breaks is an error named by its layer, which blocks every code and data output.
func TestBuildInactiveLayerBlocks(t *testing.T) {
	for _, check := range []bool{false, true} {
		fsys := rowsProject("  two { v: 2 }\n", true)
		res, err := openTierProject(t, fsys).Build(context.Background(), canon.BuildOptions{Check: check})
		if err != nil {
			t.Fatal(err)
		}
		f, ok := layerFinding(res.Check.Findings, diag.E1905.Def().Code, "dev")
		if !ok || !res.Check.HasErrors() || f.File != "a/dev.layer.canon" {
			t.Errorf("check %v: %v, want dev's %s located in its file", check, res.Check.Findings, diag.E1905.Def().Code)
		}
		if _, wrote := fsys.files[rowsOut]; wrote || len(res.Outputs) != 0 {
			t.Errorf("check %v: outputs %v, want none written or listed", check, res.Outputs)
		}
	}
}

// DECISIONS 469, EVALUATION.md 9.4: a project whose every layer evaluates builds its outputs from
// the active layers only, byte for byte what it builds without the layer file.
func TestBuildCleanLayerSameOutputs(t *testing.T) {
	outputs := map[bool][]byte{}
	for _, layered := range []bool{false, true} {
		fsys := rowsProject("  one { v: 1 }\n", layered)
		res, err := openTierProject(t, fsys).Build(context.Background(), canon.BuildOptions{})
		if err != nil || res.Check.HasErrors() {
			t.Fatalf("layered %v: %v, %v", layered, err, res.Check.Findings)
		}
		outputs[layered] = fsys.files[rowsOut]
	}
	if len(outputs[false]) == 0 || !bytes.Equal(outputs[false], outputs[true]) {
		t.Errorf("without the layer %q, with it %q: want the same output", outputs[false], outputs[true])
	}
}

// DECISIONS 449, 469, API.md B5: Test's Check carries the warnings canon check reports, so a warning
// only an inactive layer raises is in it, named by the layer; the tests read the base.
func TestTestCheckInactiveLayerWarning(t *testing.T) {
	src := "/// A.\npackage a\n\n/// A config.\nrecord Config {\n  /// Its port.\n  port: Int\n\n  warn port < 1000 else \"a high port\"\n}\n\n" +
		"/// The config.\nlet config: Config = { port: 80 }\n\ntest \"base\" {\n  expect config.port == 80\n}\n"
	fsys := newMemFS(map[string][]byte{
		"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": []byte(src),
		"/law/a/dev.layer.canon": []byte("package a\nlayer dev\n\namend config {\n  port: 9000\n}\n"),
	})
	res, err := openTierProject(t, fsys).Test(context.Background(), canon.TestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f, ok := layerFinding(res.Check.Findings, diag.W5001.Def().Code, "dev")
	if !ok || res.Check.Summary.Warnings != 1 || res.Failed != 0 || !strings.HasPrefix(f.File, "a/") {
		t.Errorf("check %v, summary %+v, failed %d: want dev's %s located, the test passing", res.Check.Findings, res.Check.Summary, res.Failed, diag.W5001.Def().Code)
	}
	if slices.ContainsFunc(res.Check.Findings, func(f canon.Finding) bool { return f.Severity == canon.SeverityError }) {
		t.Errorf("check %v: want no error", res.Check.Findings)
	}
}
