package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// API.md E19 (DECISIONS 463): a DryRun applies the inactive layers' refusal too, writing nothing.
func TestEditInactiveLayerDryRun(t *testing.T) {
	c := allLayersCases[2]
	res, err := allLayersOpen(t, nil, "").Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}, DryRun: true})
	var re *canon.RejectedError
	if !errors.As(err, &re) || res == nil || res.Applied {
		t.Fatalf("%v, want the dry run rejected", err)
	}
	if _, ok := layerFinding(re.Findings, c.code, c.layer); !ok {
		t.Errorf("rejected with %v, want %s named by layer %s", re.Findings, c.code, c.layer)
	}
}

// API.md E18 (DECISIONS 463): with a layer active, a different inactive layer the edit breaks is
// checked on top of it and named.
func TestEditInactiveLayerOverActive(t *testing.T) {
	c := allLayersCases[2]
	res, err := allLayersOpen(t, []string{"lw"}, "").Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
	var re *canon.RejectedError
	if !errors.As(err, &re) || res == nil || res.Applied {
		t.Fatalf("%v, want the edit rejected", err)
	}
	if _, ok := layerFinding(re.Findings, c.code, c.layer); !ok {
		t.Errorf("rejected with %v, want %s named by layer %s", re.Findings, c.code, c.layer)
	}
}

// importedLaw is y importing x, which declares lx, and e: y's gap reads both.
func importedLaw() canon.Options {
	return project(map[string]string{
		"project.canon":    editLaw["project.canon"],
		"x/x.canon":        "/// X.\npackage x\n\n/// A row.\nrecord R {\n  /// V.\n  v: Int = 0\n}\n\n/// Rows.\nlet xs: table R = {\n  one { v: 9 }\n}\n",
		"x/lx.layer.canon": "package x\nlayer lx\n\namend xs {\n  one.v: 1\n}\n",
		"e/e.canon":        "/// E.\npackage e\n\n/// N.\nlet n: Int = 0\n",
		"y/y.canon":        "/// Y.\npackage y\n\nimport x\nimport e\n\n/// The gap.\nlet gap: Int(0..) = x.xs.one.v - e.n\n",
	})
}

// API.md E18 (DECISIONS 463): a layer declared by a package the re-checked ones only import is
// checked too, over the re-checked packages that import its declarer.
func TestEditInactiveLayerOfImport(t *testing.T) {
	for _, active := range [][]string{nil, {"lx"}} {
		opts := importedLaw()
		opts.Layers = active
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("e:n", canon.Int(5))}})
		var re *canon.RejectedError
		code := diag.E3204.Def().Code
		switch {
		case !errors.As(err, &re) || res == nil || res.Applied:
			t.Errorf("active %v: %v, want the edit rejected", active, err)
		case len(active) == 0 && !slices.ContainsFunc(re.Findings, func(f canon.Finding) bool { return f.Code == string(code) && f.Layer == "lx" }):
			t.Errorf("rejected with %v, want %s named by layer lx", re.Findings, code)
		}
		_ = p.Close()
	}
}

// lowWarns are two warnings of d in source order; the first stops under lf (ps.one.v: 2).
const lowWarns = "\nwarn ps.one.v > 1 else \"first\"\n\nwarn k0 > 100 else \"second\"\n"

// API.md F7, E18 (DECISIONS 463): a new error an inactive layer finds is seen though its own
// analysis' cap would drop it, then the cap applies once to the merged findings: kept in F2 order,
// the dropped counted.
func TestEditInactiveLayerPastItsCap(t *testing.T) {
	const warn = "\nwarn k0 > 100 else \"low\"\n"
	res, err := allLayersOpenMax(t, nil, warn, 1).Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("d:ps[one]")}})
	var re *canon.RejectedError
	if !errors.As(err, &re) || res == nil || res.Applied {
		t.Fatalf("%v, want the edit rejected by the layer's dropped error", err)
	}
	want := []canon.Truncation{{Package: "d", Errors: 1}}
	if len(res.Findings) != 1 || res.Summary.Errors != 1 || res.Summary.Warnings != 1 || !slices.Equal(res.Summary.Truncated, want) {
		t.Errorf("findings %v, summary %+v: want the warning kept, the layer's error counted and dropped", res.Findings, res.Summary)
	}
}

// API.md F7, E18 (DECISIONS 463): a base finding the session's cap dropped is not new under an
// inactive layer: it is never reported as the layer's, nor counted twice.
func TestEditInactiveLayerBaseCapped(t *testing.T) {
	res, err := allLayersOpenMax(t, nil, lowWarns, 1).Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ps[two].v", canon.Int(7))}})
	if err != nil || !res.Applied {
		t.Fatalf("%v, want the edit written", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Layer != "" || res.Summary.Warnings != 2 || len(res.Summary.Truncated) != 1 {
		t.Errorf("findings %v, summary %+v: want the first warning kept, the second counted once, no layer named", res.Findings, res.Summary)
	}
}

// API.md F11 (DECISIONS 463): a finding new under an inactive layer about a value an active
// layer's amendment produced keeps that layer's name.
func TestEditInactiveLayerKeepsLayer(t *testing.T) {
	src := "/// D.\npackage d\n\n/// A count.\ntype Count(item: Bool) = match item {\n  false => Int(0..5)\n  true => Int(0..50)\n}\n\n" +
		"/// A row.\nrecord P {\n  /// V.\n  v: Int = 0\n  /// Items.\n  items: Bool = true\n  /// N.\n  n: Count(items) = 0\n}\n\n/// Big.\nlet big: Int = 20\n\n/// Rows.\nlet ps: table P = {\n  one { v: 1 }\n}\n"
	opts := project(map[string]string{
		"project.canon": editLaw["project.canon"], "d/d.canon": src,
		"d/la.layer.canon": "package d\nlayer la\n\namend ps {\n  one.n: big\n}\n",
		"d/lb.layer.canon": "package d\nlayer lb\n\namend ps {\n  one.items: false\n}\n",
	})
	opts.Layers = []string{"la"}
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ps[one].v", canon.Int(2))}, AllowErrors: true})
	if err != nil || !res.Applied {
		t.Fatalf("%v, want the draft written", err)
	}
	if _, ok := layerFinding(res.Findings, diag.E3204.Def().Code, "la"); !ok || len(res.Findings) != 1 {
		t.Errorf("findings %v: want lb's new range finding, named by la, whose amendment made the value", res.Findings)
	}
}
