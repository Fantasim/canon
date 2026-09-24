package ir_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	teamboardValues = "testdata/teamboard"
	irDump          = "ir.txt"
	noFindings      = "0 errors, 0 warnings"
)

// teamboard is examples/teamboard and its imports after stages A to D: the values and the
// lookup results the evaluator gives, read from testdata/teamboard.
func teamboard(t *testing.T) (*world, []*ir.Package) {
	t.Helper()
	w := newWorld(t)
	w.examples(t, "teamboard", "sovcommon/roles", "sovcommon/ui")
	entries, err := os.ReadDir(teamboardValues)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(teamboardValues, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		w.add(t, e.Name(), data)
	}
	w.calls = w.fixtureCalls
	return w, w.build(t, "teamboard", "sovcommon.roles", "sovcommon.ui")
}

// EVALUATION.md §1 phase 7, IMPLEMENTATION-PLAN §6 M1: the IR of teamboard, roles and ui, every value, lookup table and emit resolved, with no finding.
func TestTeamboardIR(t *testing.T) {
	golden.Run(t, "testdata/ir/teamboard.txtar", func(t *testing.T, _ golden.Case) []byte {
		w, pkgs := teamboard(t)
		if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
			t.Errorf("findings:\n%s", out)
		}
		var b strings.Builder
		for _, p := range pkgs {
			b.WriteString(dump(p))
		}
		return []byte(b.String())
	}, golden.Expected(irDump))
}

// WIRE.md §8: gen/json writes, from the IR stage E builds, the files its hand-built teamboard fixture gave (testdata/teamboard.txtar of gen/json); intents.json differs by design: it carries every `$fns` of the package, where the fixture kept two.
func TestTeamboardJSONMatchesTheFixture(t *testing.T) {
	_, pkgs := teamboard(t)
	p := pkgs[len(pkgs)-1]
	var jsonEmit *ir.Emit
	for _, e := range p.Emits {
		if e.Target == ir.TargetJSON {
			jsonEmit = e
		}
	}
	files, err := jsongen.Generate(p, jsonEmit)
	if err != nil {
		t.Fatal(err)
	}
	cases, err := golden.Load("../gen/json/testdata/teamboard.txtar")
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, want := range cases[0].Archive.Files {
		got, ok := fileNamed(files, want.Name)
		if !ok || want.Name == "intents.json" {
			continue
		}
		compared++
		if !bytes.Equal(got, want.Data) {
			t.Errorf("%s:\n%s\nwant\n%s", want.Name, got, want.Data)
		}
	}
	if compared != 6 {
		t.Errorf("compared %d files, want 6", compared)
	}
}

// fileNamed is the content of the generated file named name, or ok is false.
func fileNamed(files []ir.File, name string) ([]byte, bool) {
	for _, f := range files {
		if f.Path == name {
			return f.Content, true
		}
	}
	return nil, false
}
