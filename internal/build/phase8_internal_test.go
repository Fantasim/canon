package build

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
)

const (
	phase8Case   = "testdata/incremental/phase8.txtar"
	phase8Budget = 5000
	phase8Title  = "one 3"
)

// EVALUATION.md §2.1, VIEWMODEL.md X7, J5: phase 8 spends none of the counter; models are alike in either order.
func TestPhase8SpendsNothing(t *testing.T) {
	pkgs := []string{"c", "d"}
	first := phase8Models(t, pkgs)
	second := phase8Models(t, []string{"d", "c"})
	if !bytes.Contains(first["c"], []byte(phase8Title)) {
		t.Errorf("c's model lacks the title %q phase 8 renders", phase8Title)
	}
	for _, pkg := range pkgs {
		if !bytes.Equal(first[pkg], second[pkg]) {
			t.Errorf("%s's model depends on the order the models were built in", pkg)
		}
	}
}

// phase8Models builds the view models of pkgs in order on one analysis of the case, and checks
// the analysis's counter is the same after them.
func phase8Models(t *testing.T, pkgs []string) map[string][]byte {
	t.Helper()
	z := withBudget(t, phase8Case, phase8Budget)
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if counts(a, diag.E4401.Def().Code) == 0 {
		t.Fatalf("budget %d held", phase8Budget)
	}
	before := a.r.ev.Charged()
	out := map[string][]byte{}
	for _, pkg := range pkgs {
		m, err := a.ViewModel(context.Background(), pkg)
		if err != nil {
			t.Fatal(err)
		}
		if out[pkg], err = viewgen.Write(m); err != nil {
			t.Fatal(err)
		}
	}
	if after := a.r.ev.Charged(); !reflect.DeepEqual(before, after) {
		t.Errorf("phase 8 charged the counter: %v, then %v", before, after)
	}
	return out
}
