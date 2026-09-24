package golden

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// dataDisplay is the display path every positionCase mutates (WIRE.md §2.3).
const dataDisplay = "pipeline/data/II_POT_HEAL_L.json"

// positionCase mutates one value of dataDisplay and pins the finding it produces.
type positionCase struct {
	name    string
	old, mu string // byte string mutate replaces; must occur exactly once
	line    int
	col     int
	pointer string
	code    diag.Code
}

// positionCases: each row's line/col/pointer is derived from the committed file's actual
// bytes (comment on each row). The file (WIRE.md, a flat record) has no nested JSON value,
// so neither case below pins a nested position.
var positionCases = []positionCase{
	{
		// TYPES.md §7.4: nHeal decodes fine, but verify reports it outside Int(1..=100_000).
		name: "range violation",
		old:  `"nHeal": 2500`,
		mu:   `"nHeal": 200000`,
		// line 4 of the file is `  "nHeal": 2500,`; `  "nHeal": ` is 11 bytes, so the value
		// (shared by old and mu) starts at byte 12.
		line:    4,
		col:     12,
		pointer: "/nHeal",
		code:    diag.E3204.Def().Code,
	},
	{
		// WIRE.md §5.1: szName is declared String; a JSON number there is a kind mismatch.
		name: "wire kind mismatch",
		old:  `"szName": "IDS_PROPITEM_TXT_POT_L"`,
		mu:   `"szName": 42`,
		// line 3 of the file is `  "szName": "IDS_...`; `  "szName": ` is 12 bytes, so the
		// value starts at byte 13.
		line:    3,
		col:     13,
		pointer: "/szName",
		code:    diag.E7110.Def().Code,
	},
}

// TestFindingPositionInLoadedJSON pins a finding's line, column, pointer (IMPLEMENTATION-PLAN.md §6 M2) per positionCases.
func TestFindingPositionInLoadedJSON(t *testing.T) {
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range positionCases {
		t.Run(c.name, func(t *testing.T) { runPositionCase(t, root, c) })
	}
}

// runPositionCase copies examples/ (the committed file is never edited), applies one mutation,
// runs `canon check pipeline` as TestExamples does, and compares the one finding it produces.
func runPositionCase(t *testing.T, root string, c positionCase) {
	t.Helper()
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "proj")
	if err := copyProject(proj, root); err != nil {
		t.Fatal(err)
	}
	mutateJSON(t, proj, c.old, c.mu)
	roots := exampleRoots(proj, tmp)
	p, err := canon.Open(filepath.ToSlash(proj), canon.Options{Roots: roots, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	checked, err := p.Check(context.Background(), "pipeline")
	if err != nil {
		t.Fatal(err)
	}
	f := onlyFinding(t, checked.Findings)
	want := fmt.Sprintf("%s %s:%d:%d %s", c.code, dataDisplay, c.line, c.col, c.pointer)
	got := fmt.Sprintf("%s %s:%d:%d %s", f.Code, f.File, f.Line, f.Col, f.Pointer)
	if got != want {
		t.Errorf("finding location: got %q, want %q\nfull finding: %+v", got, want, f)
	}
}

// mutateJSON replaces old with mu in proj's copy of dataDisplay; old must occur exactly once.
func mutateJSON(t *testing.T, proj, old, mu string) {
	t.Helper()
	path := filepath.Join(proj, filepath.FromSlash(dataDisplay))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(data, []byte(old)); n != 1 {
		t.Fatalf("%s: %q occurs %d times, want 1", dataDisplay, old, n)
	}
	mutated := bytes.Replace(data, []byte(old), []byte(mu), 1)
	if err := os.WriteFile(path, mutated, filePerm); err != nil {
		t.Fatal(err)
	}
}

// onlyFinding fails unless findings holds exactly one finding, and returns it.
func onlyFinding(t *testing.T, findings []canon.Finding) canon.Finding {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	return findings[0]
}
