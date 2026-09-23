package ratchet

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

func fnLen(file, sym string, lines, at int) finding.Finding {
	return finding.Finding{Rule: "fn-length", File: file, Symbol: sym, Value: lines, Line: at}
}

func always(string) bool { return true }

func modeOf(id string) rules.Mode {
	rl, _ := rules.Lookup(id)
	return rl.Mode
}

func TestCompare(t *testing.T) {
	base := Build([]finding.Finding{fnLen("a.go", "F", 70, 10), fnLen("a.go", "G", 80, 50)})
	tests := []struct {
		name                   string
		cur                    []finding.Finding
		wantNew, wantGrew, fix int
	}{
		{"unchanged, lines moved", []finding.Finding{fnLen("a.go", "F", 70, 90), fnLen("a.go", "G", 80, 5)}, 0, 0, 0},
		{"one grew", []finding.Finding{fnLen("a.go", "F", 75, 10), fnLen("a.go", "G", 80, 50)}, 0, 1, 0},
		{"one new", []finding.Finding{fnLen("a.go", "F", 70, 10), fnLen("a.go", "G", 80, 50), fnLen("b.go", "H", 61, 1)}, 1, 0, 0},
		{"one fixed", []finding.Finding{fnLen("a.go", "F", 70, 10)}, 0, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Compare(tt.cur, base, modeOf, always)
			if len(v.New) != tt.wantNew || len(v.Grew) != tt.wantGrew || v.Fixed != tt.fix {
				t.Fatalf("new=%d grew=%d fixed=%d, want %d %d %d", len(v.New), len(v.Grew), v.Fixed, tt.wantNew, tt.wantGrew, tt.fix)
			}
		})
	}
}

func TestCompareEnforceIgnoresBaseline(t *testing.T) {
	f := finding.Finding{Rule: "err-style", File: "a.go"}
	v := Compare([]finding.Finding{f}, Build([]finding.Finding{f}), modeOf, always)
	if len(v.Enforced) != 1 || !v.Failed() {
		t.Fatalf("enforce rule must fail even when baselined: %+v", v)
	}
}

func TestCompareScope(t *testing.T) {
	cur := []finding.Finding{fnLen("b.go", "H", 61, 1)}
	onlyA := func(p string) bool { return p == "a.go" }
	if v := Compare(cur, Baseline{}, modeOf, onlyA); v.Failed() {
		t.Fatalf("out-of-scope finding judged: %+v", v)
	}
}

func TestTightenNeverRaises(t *testing.T) {
	base := Build([]finding.Finding{fnLen("a.go", "F", 70, 1), fnLen("a.go", "G", 80, 1)})
	cur := []finding.Finding{fnLen("a.go", "F", 90, 1), fnLen("c.go", "N", 99, 1)}
	nb, n := Tighten(base, cur, always)
	if len(nb) != 1 || n != 1 {
		t.Fatalf("got %d entries, %d changed", len(nb), n)
	}
	for _, e := range nb {
		if e.Value != 70 {
			t.Fatalf("tighten raised F to %d", e.Value)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	b := Build([]finding.Finding{fnLen("a.go", "F", 70, 1), {Rule: "magic-string", File: "x", Detail: "tab\there"}})
	p := filepath.Join(t.TempDir(), "sub", "baseline.tsv")
	if err := b.Save(p); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Load(p)
	if err != nil || !ok || len(got) != len(b) {
		t.Fatalf("round trip: %v %v %d", err, ok, len(got))
	}
}
