//go:build linux

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// flagEdit is the size of the benchmark project TestBenchEdit measures; make bench-edit sets it.
var flagEdit = flag.Int("benchgen.edit", 0, "run TestBenchEdit on a benchmark project of n entries and the examples (make bench-edit)")

// The measures of IMPLEMENTATION-PLAN.md §7.6's NFR-01 table, their targets in decimal units.
const (
	benchEdits      = 200 // accepted Sets whose p95 is measured
	benchRejectPct  = 10  // rejected Sets past this share of all Sets fail the run (sampling bias)
	benchPercent    = 100
	benchPercentile = 95
	benchSeed       = 1
	targetCold      = 10 * time.Second
	targetWarm      = time.Second
	targetEdit      = 300 * time.Millisecond
	targetEval      = 150 * time.Millisecond
	targetRSS       = 1_500_000_000 // bytes, 1.5 GB
	targetVM        = 5_000_000     // bytes, 5 MB, without the search index and findings (VIEWMODEL.md J16)
	vmSearch        = "search"
	vmFindings      = "findings"
	vmValues        = "values"
	vmTypes         = "types"
	vmAssets        = "assets"
	notGated        = "reported, not gated (log M4 U7b)"
	unitSeconds     = "s"
)

// benchTarget is one project TestBenchEdit measures: its directory and the roots it redirects.
type benchTarget struct {
	name    string
	dir     string
	roots   map[string]string
	guarded bool // the benchmark: shares and skipped kinds fail it; the examples report them
}

// NFR-01 (M4 acceptance item 4): cold canon check and its peak RSS per project, the p95s of an
// Edit and of an Evaluate after it, and each package's view model, each against its target;
// warm canon check reported; printed with the machine (log-2026-09-29 M4: the reference).
func TestBenchEdit(t *testing.T) {
	if *flagEdit <= 0 {
		t.Skip("guarded by -benchgen.edit: run with `make bench-edit`")
	}
	t.Log(machine())
	bin := buildCanon(t)
	bench := filepath.Join(t.TempDir(), "bench")
	if err := generate(benchSeed, bench, *flagEdit); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, target := range []benchTarget{{name: "bench (n=" + strconv.Itoa(*flagEdit) + ")", dir: bench, guarded: true}, examplesCopy(t)} {
		t.Run(target.name, func(t *testing.T) {
			var r report
			defer r.print(t)
			normalize(t, bin, target)
			p := openClean(t, target)
			coldChecks(t, bin, target, units(t, p, target), &r)
			editTimes(t, p, &r, target)
		})
	}
}

// openClean opens the target in process and refuses one with an error: a run on a project
// with errors is not measured (log-2026-09-29 M4 U7b-r).
func openClean(t *testing.T, target benchTarget) *canon.Project {
	t.Helper()
	p, err := canon.Open(target.dir, canon.Options{Roots: target.roots})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Check(context.Background())
	if err != nil || res.HasErrors() {
		t.Fatalf("Check: %v, %d errors; a project with errors is not measured", err, res.Summary.Errors)
	}
	return p
}

// units are what the cold check measures one by one: the benchmark whole, each example alone.
func units(t *testing.T, p *canon.Project, target benchTarget) []string {
	t.Helper()
	if target.roots == nil {
		return []string{""}
	}
	pkgs, err := p.Packages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		out[i] = pkg.Name
	}
	return out
}

// report is one target's lines, in the order of §7.6's table.
type report struct {
	lines []line
}

// line is one printed line: a name and its measures.
type line struct {
	name     string
	measures []measure
}

// measure is one number against its target; an ungated one is printed, never failed.
type measure struct {
	value, limit float64
	unit         string
	gated        bool
}

func (r *report) add(name string, ms ...measure) {
	r.lines = append(r.lines, line{name: name, measures: ms})
}

func seconds(d, limit time.Duration, gated bool) measure {
	return measure{value: d.Seconds(), limit: limit.Seconds(), unit: unitSeconds, gated: gated}
}

// print logs every line and fails for each gated measure past its target.
func (r *report) print(t *testing.T) {
	t.Helper()
	for _, l := range r.lines {
		cells := make([]string, len(l.measures))
		for i, m := range l.measures {
			cells[i] = fmt.Sprintf("%9.3f %-2s (<= %.3f) %s", m.value, m.unit, m.limit, verdict(m))
			if m.gated && m.value > m.limit {
				t.Errorf("%s: %.3f %s, target %.3f %s", l.name, m.value, m.unit, m.limit, m.unit)
			}
		}
		t.Logf("%-48s %s", l.name, strings.Join(cells, "  "))
	}
}

func verdict(m measure) string {
	switch {
	case !m.gated:
		return notGated
	case m.value > m.limit:
		return "EXCEEDED"
	}
	return "ok"
}

// vmFacts are what the view models tell the Sets: the values they name (VIEWMODEL.md J8),
// sorted; every enum's live members; each asset root's directory, project-relative.
type vmFacts struct {
	roots  []string
	enums  map[string][]string
	assets map[string]string
}

// viewModels measures each package's view model without its search index and findings, and
// gathers its facts.
func viewModels(t *testing.T, p *canon.Project, r *report) vmFacts {
	ctx := context.Background()
	pkgs, err := p.Packages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	facts := vmFacts{enums: map[string][]string{}, assets: map[string]string{}}
	for _, pkg := range pkgs {
		m, err := p.ViewModel(ctx, pkg.Name)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(m.JSON(), &doc); err != nil {
			t.Fatal(err)
		}
		size := len(m.JSON()) - len(doc[vmSearch]) - len(doc[vmFindings])
		r.add("view model of "+pkg.Name+" (MB)", measure{value: float64(size) / 1e6, limit: targetVM / 1e6, unit: "MB", gated: true})
		facts.roots = append(facts.roots, valueNames(doc[vmValues])...)
		enumMembers(doc[vmTypes], facts.enums)
		assetDirs(doc[vmAssets], facts.assets)
	}
	slices.Sort(facts.roots)
	return facts
}

// assetDirs adds each asset root of a view model with its directory (VIEWMODEL.md J-assets).
func assetDirs(raw json.RawMessage, into map[string]string) {
	var assets map[string]struct {
		Dir string `json:"dir"`
	}
	if err := json.Unmarshal(raw, &assets); err != nil {
		return
	}
	//canon:unordered each root is stored under its own name
	for root, a := range assets {
		into[root] = a.Dir
	}
}

func valueNames(raw json.RawMessage) []string {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return slices.Collect(maps.Keys(values)) // the caller sorts them
}

// enumMembers adds each enum of a view model's types, by qualified name, with its members that
// are not retired.
func enumMembers(raw json.RawMessage, into map[string][]string) {
	var types map[string]struct {
		Kind    string `json:"kind"`
		Members []struct {
			Name    string `json:"name"`
			Retired bool   `json:"retired"`
		} `json:"members"`
	}
	if err := json.Unmarshal(raw, &types); err != nil {
		return
	}
	//canon:unordered each enum is stored under its own name
	for name, def := range types {
		for _, m := range def.Members {
			if !m.Retired {
				into[name] = append(into[name], m.Name)
			}
		}
	}
}

// p95 is the nearest-rank 95th percentile; 0 for no sample.
func p95(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Sorted(slices.Values(ds))
	return s[(len(s)*benchPercentile+benchPercent-1)/benchPercent-1]
}
