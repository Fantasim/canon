package build

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	precomputedCase     = "testdata/builds/precomputed_dependent.txtar"
	precomputedPoisoned = "testdata/builds/precomputed_poisoned.txtar"
	precomputedInvalid  = "testdata/findings/E3802_precomputed.txtar"
	precomputedRec      = "Hold"
	precomputedBonus    = 1 // Perk.bonus
)

// DECISIONS 324, EVALUATION.md §2.3, TYPES.md §11.6: `bonus: loud` reaches the IR as Tone.loud.
func TestPrecomputedConverted(t *testing.T) {
	hold := irRecord(t, analyzed(t, precomputedCase), precomputedRec)
	cases := []struct {
		fn   string
		want []string // the bonus of each result, the cells in domain order
	}{
		{"perk", []string{"loud"}},
		{"toned", []string{"quiet", "loud"}},
	}
	for _, c := range cases {
		got := bonuses(t, hold, c.fn)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: bonuses %v, want members %v", c.fn, got, c.want)
		}
	}
}

// DECISIONS 324, EVALUATION.md §2.3, §7.1: an invalid or poisoned result is a missing receiver or cell in the IR.
func TestPrecomputedMissing(t *testing.T) {
	cases := []struct {
		file string
		fn   string
		want int // the receivers holding a result
	}{
		{precomputedInvalid, "bad", 0},   // E3802
		{precomputedInvalid, "pick", 0},  // its count cell is E3802: the receiver's table is missing
		{precomputedPoisoned, "perk", 1}, // h's predicate divides by zero; i's holds
		{precomputedCase, "perk", 1},
	}
	for _, c := range cases {
		hold := irRecord(t, analyzed(t, c.file), precomputedRec)
		i := slices.IndexFunc(hold.Methods, func(m *ir.ExportFn) bool { return m.Name == c.fn })
		if i < 0 {
			t.Fatalf("%s: no method %s", c.file, c.fn)
		}
		if got := len(hold.Methods[i].Instances); got != c.want {
			t.Errorf("%s %s: %d receivers hold a result, want %d", c.file, c.fn, got, c.want)
		}
	}
}

// analyzed is the IR stage E builds for the case file.
func analyzed(t *testing.T, file string) []*ir.Package {
	t.Helper()
	z := archiveAnalyzer(t, file)
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r, err := p.prepare(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.analyze(ctx); err != nil {
		t.Fatal(err)
	}
	return r.ir
}

// irRecord is the record name of the IR's packages.
func irRecord(t *testing.T, pkgs []*ir.Package, name string) *ir.Record {
	t.Helper()
	for _, p := range pkgs {
		for _, ty := range p.Types {
			if rec, ok := ty.(*ir.Record); ok && rec.Name == name {
				return rec
			}
		}
	}
	t.Fatalf("no record %s in the IR", name)
	return nil
}

// bonuses is the member name of the bonus of each result fn stored on rec's one receiver; a
// value of any other kind fails the test.
func bonuses(t *testing.T, rec *ir.Record, fn string) []string {
	t.Helper()
	i := slices.IndexFunc(rec.Methods, func(m *ir.ExportFn) bool { return m.Name == fn })
	if i < 0 || len(rec.Methods[i].Instances) != 1 {
		t.Fatalf("%s: no single stored instance", fn)
	}
	in := rec.Methods[i].Instances[0]
	results := []value.Value{in.Result}
	if in.Table != nil {
		results = in.Table.Cells
	}
	var out []string
	for _, v := range results {
		perk, ok := v.(*value.Record)
		if !ok {
			t.Fatalf("%s: result %T, want a record", fn, v)
		}
		m, ok := perk.Fields[precomputedBonus].(*value.Member)
		if !ok {
			t.Fatalf("%s: bonus %T, want *value.Member", fn, perk.Fields[precomputedBonus])
		}
		out = append(out, m.Enum.Members[m.Index].Name)
	}
	return out
}
