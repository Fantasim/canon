package cppgen

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"golang.org/x/tools/txtar"
)

// TestE8302MatchesFindingsTxtar is CODEGEN.md §5.12: the getter of E8302_1.txtar's package signals its findings.txt message.
func TestE8302MatchesFindingsTxtar(t *testing.T) {
	a, err := txtar.ParseFile("testdata/findings/E8302_1.txtar")
	if err != nil {
		t.Fatal(err)
	}
	var line string
	for _, f := range a.Files {
		if f.Name == "findings.txt" {
			line = strings.TrimSuffix(string(f.Data), "\n")
		}
	}
	prefix := string(diag.E8302.Def().Code) + ": "
	want, ok := strings.CutPrefix(line, prefix)
	if !ok {
		t.Fatalf("testdata/findings/E8302_1.txtar: findings.txt %q has no %s prefix", line, prefix)
	}

	rec := &ir.Record{Pkg: "a", Name: "Config", Fields: []*ir.Field{
		{Name: "apiKey", WirePath: []string{"apiKey"}, Type: ir.TypeRef{Kind: types.String}, Optional: true, Input: &types.Input{Env: "A_API_KEY"}},
	}}
	e := &ir.Emit{Target: ir.TargetCpp, Dir: "a/out", Mode: ir.ModeData, Namespace: "a"}
	p := &ir.Package{Name: "a", Dir: "a", Types: []ir.Type{rec}, Emits: []*ir.Emit{e}}
	files, err := Generate(p, e)
	if err != nil {
		t.Fatal(err)
	}
	var header string
	for _, f := range files {
		if f.Path == "a.gen.h" {
			header = string(f.Content)
		}
	}
	if !strings.Contains(header, quote(want)) {
		t.Errorf("a.gen.h does not contain %s", quote(want))
	}
}
