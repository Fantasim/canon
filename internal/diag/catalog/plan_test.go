package catalog

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// IMPLEMENTATION-PLAN.md §3: table order, rows outside internal/, sub-packages of a row.
func TestPackagesReadsThePlan(t *testing.T) {
	_, plan := readSpec(t)
	pkgs, err := Packages(plan)
	if err != nil {
		t.Fatal(err)
	}
	if pkgs[0].Name != "source" || pkgs[len(pkgs)-1].Name != "testkit" {
		t.Errorf("first %s, last %s", pkgs[0].Name, pkgs[len(pkgs)-1].Name)
	}
	byName := map[string]Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}
	checks := []bool{
		byName["api"].Dir == "api",
		byName["cmd/canon"].Dir == "cmd/canon",
		byName["gen/json"].Dir == "internal/gen/json",
		slices.Equal(byName["eval"].Subs, []string{"internal/eval/std"}),
	}
	if slices.Contains(checks, false) {
		t.Errorf("rows read as %+v", pkgs)
	}
}

func TestPackagesRefusesADuplicateRow(t *testing.T) {
	_, plan := readSpec(t)
	row := "| `lsp` | language server | §8.4 | workspace, check, format |"
	_, err := Packages([]byte(strings.Replace(string(plan), row, row+"\n"+row, 1)))
	if !errors.Is(err, errPlan) {
		t.Errorf("got %v, want %v", err, errPlan)
	}
}
