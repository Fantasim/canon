package rules_test

import (
	"slices"
	"testing"

	"golang.org/x/tools/txtar"
)

// API.md F1: a check run on an instance is given the instance's canonical path, the Path of a
// hard error the run raises; a package check, having no instance, is given none.
func TestInstanceCheckRunsGetInstancePath(t *testing.T) {
	a, err := txtar.ParseFile("testdata/findings/E5001_1.txtar")
	if err != nil {
		t.Fatal(err)
	}
	fx := fromArchive(t, a)
	oneLineCase(fx)
	fx.run()
	got := slices.Clone(fx.ev.paths)
	slices.Sort(got)
	got = slices.Compact(got)
	want := []string{"", "columns.archive", "columns.flaky", "columns.unclaimed"}
	if !slices.Equal(got, want) {
		t.Errorf("paths given to runs: %q, want %q", got, want)
	}
}
