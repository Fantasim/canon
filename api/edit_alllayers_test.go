package canon_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// allLayersSrc is driverSrc with qs[a] giving no rewards of its own, and ps, rows no dependent
// type reads.
var allLayersSrc = strings.Replace(driverSrc, `a { items: true, rewards: ["s"] }`, `a { items: true }`, 1) +
	"\n/// A row.\nrecord P {\n  /// V.\n  v: Int = 0\n}\n\n/// Rows.\nlet ps: table P = {\n  one { v: 1 }\n  two { v: 2 }\n}\n"

// allLayersOpen opens allLayersSrc with its layers lw (on qs) and lf (on ps), layers active,
// more added to its main file.
func allLayersOpen(t *testing.T, layers []string, more string) *canon.Project {
	t.Helper()
	return allLayersOpenMax(t, layers, more, 0)
}

// allLayersOpenMax is allLayersOpen keeping max findings per package (API.md F7), 0 the default.
func allLayersOpenMax(t *testing.T, layers []string, more string, max int) *canon.Project {
	t.Helper()
	opts := project(map[string]string{
		"project.canon": editLaw["project.canon"], "d/d.canon": allLayersSrc + more,
		"d/lw.layer.canon": "package d\nlayer lw\n\namend qs {\n  a.rewards: [\"s\"]\n}\n",
		"d/lf.layer.canon": "package d\nlayer lf\n\namend ps {\n  one.v: 2\n}\n",
	})
	opts.Layers, opts.MaxFindings = layers, max
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// allLayersCases are base edits that break a layer the session does not apply.
var allLayersCases = []struct {
	name, layer string
	code        diag.Code
	op          canon.Op
}{
	{"a driver Set leaves the layer's rewards in error", "lw", diag.E3802.Def().Code, canon.Set("d:qs[a].items", canon.Bool(false))},
	{"a Remove of the entry the layer amends", "lw", diag.E1905.Def().Code, canon.Remove("d:qs[a]")},
	{"a Remove of a non-dependent entry the layer amends", "lf", diag.E1905.Def().Code, canon.Remove("d:ps[one]")},
}

// layerFinding is the first finding of fs with code, named by layer.
func layerFinding(fs []canon.Finding, code diag.Code, layer string) (canon.Finding, bool) {
	i := slices.IndexFunc(fs, func(f canon.Finding) bool { return f.Code == string(code) && f.Layer == layer })
	if i < 0 {
		return canon.Finding{}, false
	}
	return fs[i], true
}

// API.md E18, E19, F11 (DECISIONS 463): an edit's re-check covers every layer the
// re-checked packages declare, one at a time on the active stack; a base edit that breaks an
// inactive layer is refused as for an active one, its finding named by the layer.
func TestEditInactiveLayerRefused(t *testing.T) {
	for _, c := range allLayersCases {
		for _, active := range [][]string{nil, {c.layer}} {
			res, err := allLayersOpen(t, active, "").Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
			var re *canon.RejectedError
			if !errors.As(err, &re) || res == nil || res.Applied {
				t.Errorf("%s, active %v: %v, want the edit rejected by its re-check", c.name, active, err)
				continue
			}
			if len(active) > 0 {
				continue // the active layer's own finding: its Layer is eval's (EVALUATION.md §13)
			}
			f, ok := layerFinding(re.Findings, c.code, c.layer)
			if !ok {
				t.Errorf("%s: rejected with %v, want %s named by layer %s", c.name, re.Findings, c.code, c.layer)
				continue
			}
			var buf bytes.Buffer
			if err := canon.WriteFindings(&buf, []canon.Finding{f}, canon.WriteOptions{Golden: true}); err != nil {
				t.Fatal(err)
			}
			at := "error[" + string(c.code) + "]  d/" + c.layer + ".layer.canon:"
			if text := buf.String(); !strings.HasPrefix(text, at) || !strings.Contains(text, "\n  set by layer "+c.layer+"\n") {
				t.Errorf("API.md F9, F11: %s: %q, want it located in its layer's file and its layer named", c.name, text)
			}
		}
	}
}

// API.md E19 (DECISIONS 463): with AllowErrors, an edit that breaks an inactive layer is
// written and reports the layer's finding.
func TestEditInactiveLayerReported(t *testing.T) {
	c := allLayersCases[2]
	res, err := allLayersOpen(t, nil, "").Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}, AllowErrors: true})
	if err != nil || !res.Applied {
		t.Fatalf("%v, want the draft written", err)
	}
	if _, ok := layerFinding(res.Findings, c.code, c.layer); !ok || res.Summary.Errors != 1 {
		t.Errorf("findings %v, summary %+v: want one %s named by layer %s", res.Findings, res.Summary, c.code, c.layer)
	}
}

// API.md E18: an edit that breaks no layer, active or not, passes with no finding.
func TestEditBreaksNoLayer(t *testing.T) {
	for _, active := range [][]string{nil, {"lw"}, {"lf", "lw"}} {
		res, err := allLayersOpen(t, active, "").Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ps[two].v", canon.Int(7))}})
		if err != nil || !res.Applied || len(res.Findings) != 0 {
			t.Errorf("active %v: %v, findings %v, want the edit written clean", active, err, res.Findings)
		}
	}
}

// API.md E18 (DECISIONS 463): a finding the session's own analysis has is not repeated
// by an inactive layer's: a base error present with and without the layer is reported once.
func TestEditInactiveLayerNoDuplicate(t *testing.T) {
	const bad = "\n/// A count that breaks its range.\nlet bad: Int(1..) = 0\n"
	res, err := allLayersOpen(t, nil, bad).Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ps[two].v", canon.Int(7))}, AllowErrors: true})
	if err != nil || !res.Applied {
		t.Fatalf("%v, want the draft written", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Layer != "" || res.Summary.Errors != 1 {
		t.Errorf("findings %v, summary %+v: want the base error once, named by no layer", res.Findings, res.Summary)
	}
}

// API.md E18, F9, F11: two inactive layers the same edit breaks each report their own finding,
// located in their own file, though each layer's analysis has its own file set.
func TestEditTwoInactiveLayersEachNamed(t *testing.T) {
	opts := project(map[string]string{
		"project.canon": editLaw["project.canon"], "d/d.canon": allLayersSrc,
		"d/lf.layer.canon": "package d\nlayer lf\n\namend ps {\n  one.v: 2\n}\n",
		"d/lg.layer.canon": "package d\nlayer lg\n\n// g.\namend ps {\n  one.v: 3\n}\n",
	})
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("d:ps[one]")}, AllowErrors: true})
	if err != nil || !res.Applied {
		t.Fatalf("%v, want the draft written", err)
	}
	code := diag.E1905.Def().Code
	want := []struct {
		layer string
		line  int
	}{{"lf", 5}, {"lg", 6}}
	for _, w := range want {
		f, ok := layerFinding(res.Findings, code, w.layer)
		if !ok || f.File != "d/"+w.layer+".layer.canon" || f.Line != w.line {
			t.Errorf("layer %s: %+v, want its %s at d/%s.layer.canon:%d", w.layer, f, code, w.layer, w.line)
		}
	}
	if len(res.Findings) != len(want) || res.Summary.Errors != len(want) {
		t.Errorf("findings %v, summary %+v: want one per layer", res.Findings, res.Summary)
	}
}
