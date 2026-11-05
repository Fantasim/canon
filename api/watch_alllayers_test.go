package canon_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// watchLayersLaw is package d of allLayersSrc with checkLayerFiles' layer lf, which amends ps[one].
var watchLayersLaw = map[string]string{
	"project.canon": editLaw["project.canon"], "d/d.canon": allLayersSrc, "d/lf.layer.canon": checkLayerFiles["d/lf.layer.canon"],
}

// watchImporter is a package e reading a row of d.ps.
var watchImporter = map[string]string{"e/e.canon": "/// E.\npackage e\n\nimport d\n\n/// A row's value.\nlet n: Int = d.ps.two.v\n"}

// watchBad is a base error of d, present with and without lf.
const watchBad = "\n/// A count that breaks its range.\nlet bad: Int(1..) = 0\n"

// watchLayers is watchLayersLaw with more files and d's source src, keeping max findings per
// package (0 the default), opened, checked and watched, no layer active.
func watchLayers(t *testing.T, src string, more map[string]string, max int) (*canon.Project, canon.Options, <-chan canon.Event) {
	t.Helper()
	files := maps.Clone(watchLayersLaw)
	maps.Copy(files, more)
	files["d/d.canon"] = src
	opts := project(files)
	opts.MaxFindings = max
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if _, err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	events, _ := watch(t, p, nil)
	return p, opts, events
}

// eventOfD is the next event that re-checks d.
func eventOfD(t *testing.T, events <-chan canon.Event) canon.Event {
	t.Helper()
	for {
		if ev := nextEvent(t, events); slices.Contains(ev.Packages, "d") {
			return ev
		}
	}
}

// sameAsCheck reports where ev's findings or summary differ from Check's of its packages.
func sameAsCheck(t *testing.T, p *canon.Project, ev canon.Event) {
	t.Helper()
	res := mustCheck(t, p, ev.Packages...)
	if !slices.EqualFunc(res.Findings, ev.Findings, sameFinding) {
		t.Errorf("event %v, Check %v: want the same findings", ev.Findings, res.Findings)
	}
	s, e := res.Summary, ev.Summary
	if s.Errors != e.Errors || s.Warnings != e.Warnings || s.Packages != e.Packages || !slices.Equal(s.Truncated, e.Truncated) {
		t.Errorf("event summary %+v, Check's %+v: want the same", e, s)
	}
}

// DECISIONS 469, API.md W13: a change on disk that breaks an inactive layer gives an event whose
// findings hold the layer's, as Check reports them; the fix gives an event with none.
func TestWatchDiskChangeInactiveLayer(t *testing.T) {
	p, opts, events := watchLayers(t, allLayersSrc, nil, 0)
	writeLaw(t, opts.FS, "d/d.canon", strings.Replace(allLayersSrc, "  one { v: 1 }\n", "", 1))
	ev := eventOfD(t, events)
	code := diag.E1905.Def().Code
	if _, ok := layerFinding(ev.Findings, code, "lf"); !ok || ev.Summary.Errors != 1 || ev.Err != nil {
		t.Errorf("the broken layer's event: %v, summary %+v, err %v: want lf's %s", ev.Findings, ev.Summary, ev.Err, code)
	}
	sameAsCheck(t, p, ev)
	writeLaw(t, opts.FS, "d/d.canon", allLayersSrc)
	if ev = eventOfD(t, events); len(ev.Findings) != 0 || ev.Summary.Errors != 0 {
		t.Errorf("the fixed base's event: %v, want no finding", ev.Findings)
	}
}

// watchEditCases are projects whose second edit, Remove of the row lf amends, breaks lf.
var watchEditCases = []struct {
	name, src string
	more      map[string]string
	max       int
}{
	{"d alone", allLayersSrc, nil, 0},
	{"an importer e", allLayersSrc, watchImporter, 0},
	{"one finding kept per package", allLayersSrc + watchBad, nil, 1},
}

// DECISIONS 463, 469, API.md W13, W15, E19, F7: after a first edit, an edit written with
// AllowErrors that breaks an inactive layer gives an edit event carrying the layer's finding as
// Check reports it, findings and summary alike, importers and the cut included.
func TestWatchEditInactiveLayer(t *testing.T) {
	code := diag.E1905.Def().Code
	for _, c := range watchEditCases {
		p, _, events := watchLayers(t, c.src, c.more, c.max)
		var ev canon.Event
		for _, op := range []canon.Op{canon.Set("d:ps[two].v", canon.Int(7)), canon.Remove("d:ps[one]")} { // a warm-up, then the break
			res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}, AllowErrors: true})
			if err != nil || !res.Applied {
				t.Fatalf("%s: %v, want the draft written", c.name, err)
			}
			if ev = eventOfD(t, events); ev.Cause != canon.CauseEdit {
				t.Fatalf("%s: event %s, want the edit's", c.name, ev.Cause)
			}
		}
		if _, ok := layerFinding(ev.Findings, code, "lf"); !ok && c.max == 0 {
			t.Errorf("%s: the edit's event %v: want lf's %s", c.name, ev.Findings, code)
		}
		if c.max > 0 && (len(ev.Findings) != c.max || ev.Summary.Errors != 2 || len(ev.Summary.Truncated) != 1) {
			t.Errorf("%s: %v, summary %+v: want the base's and lf's errors cut once to %d", c.name, ev.Findings, ev.Summary, c.max)
		}
		sameAsCheck(t, p, ev)
	}
}

// DECISIONS 469, API.md W13: an edit that breaks no layer gives an edit event with no finding.
func TestWatchEditCleanLayers(t *testing.T) {
	p, _, events := watchLayers(t, allLayersSrc, nil, 0)
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ps[two].v", canon.Int(7))}})
	if err != nil || !res.Applied {
		t.Fatalf("%v, want the edit written", err)
	}
	if ev := eventOfD(t, events); ev.Cause != canon.CauseEdit || len(ev.Findings) != 0 || ev.Summary.Errors != 0 {
		t.Errorf("the edit's event: %s %v, want no finding", ev.Cause, ev.Findings)
	}
}

// sameFinding compares two findings by what they print.
func sameFinding(a, b canon.Finding) bool {
	return a.Code == b.Code && a.File == b.File && a.Line == b.Line && a.Col == b.Col && a.Message == b.Message && a.Layer == b.Layer
}
