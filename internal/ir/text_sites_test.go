package ir_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	sitesLib      = "package b\n\n/// Colors.\nenum Color { x, y }\n\n/// A key.\ntype K = Color | \"x\"\n\n/// Inner.\nrecord Inner {\n  /// Wait.\n  wait: Duration @json(unit: s)\n}\n\n/// The map.\nlet m: {K: Int} = {}\n"
	sitesImporter = "package %s\n\nimport b { m, K, Inner }\n\n/// Imported.\n@text(\"clash.json\")\nexport fn clash() -> {K: Int} { return m }\n\n/// Own.\n@text(\"own.json\")\nexport fn own() -> {K: Int} { return { \"x\": 1 } }\n\n/// Again.\n@text(\"again.json\")\nexport fn again() -> {K: Int} { return { \"x\": 1 } }\n\n/// Slow.\n@text(\"slow.json\")\nexport fn slow() -> Inner { return { wait: 1500ms } }\n\nemit text { out: \"out/\" }\n"
	sitesLet      = "b.m"
	sitesSlow     = "slow"
	sitesClash    = "clash"
)

// readCounter is a world's host that counts the let values stage E reads.
type readCounter struct {
	*world
	reads map[string]int
}

func (h *readCounter) Value(ctx context.Context, pkg, name string) (value.Value, bool) {
	h.reads[pkg+"."+name]++
	return h.world.Value(ctx, pkg, name)
}

// textSites builds the library b and a text-emitting importer per name of importers: its
// clash returns b's let m (holding clashing keys, verified by stage B), own and again one
// shared clashing map, slow a record finer than its unit. It returns the findings and the reads of b.m.
func textSites(t *testing.T, importers ...string) (string, int) {
	t.Helper()
	w := newWorld(t)
	w.add(t, "b/b.canon", []byte(sitesLib))
	w.add(t, sitesLet+jsonExt, []byte(`{ "x": 1 }`))
	for _, p := range importers {
		w.add(t, p+"/"+p+canonExt, []byte(fmt.Sprintf(sitesImporter, p)))
		w.add(t, p+"."+sitesSlow+".calls"+jsonExt, []byte(`{ "": { "wait": 1.5 } }`))
	}
	w.check(t)
	stored, ok := w.Value(context.Background(), "b", "m")
	m, isMap := stored.(*value.Map)
	if !ok || !isMap {
		t.Fatalf("b.m = %v, %v", stored, ok)
	}
	m.Keys, m.Vals = append(m.Keys, colorMember(t, w, "b", "x")), append(m.Vals, intVal(2))
	shared := &value.Map{
		T:    m.T,
		Keys: []value.Value{colorMember(t, w, "b", "x"), &value.Str{V: "x", T: types.StringType}},
		Vals: []value.Value{intVal(1), intVal(2)},
	}
	w.calls = func(fn check.Object, recv value.Value, args []value.Value) (value.Value, bool) {
		switch fn.Name() {
		case sitesSlow:
			return w.fixtureCalls(fn, recv, args)
		case sitesClash:
			return m, true
		}
		return shared, true
	}
	h := &readCounter{world: w, reads: map[string]int{}}
	ir.Build(context.Background(), ir.Input{Program: w.prog, Project: w.proj, Bags: w.bags, Host: h, Fold: folder{}})
	return w.findings(t), h.reads[sitesLet]
}

// WIRE.md §5.8, §8.5, DECISIONS 308: per package, verified lets skipped and each composite walked once; the lets' set built once per stage.
func TestTextResultsAcrossSites(t *testing.T) {
	one, oneReads := textSites(t, "a")
	two, twoReads := textSites(t, "a", "c")
	codes := []struct {
		code      diag.Code
		one, both int
	}{
		{diag.E3317.Def().Code, 1, 2},
		{diag.E8102.Def().Code, 1, 2},
	}
	for _, c := range codes {
		if n := strings.Count(one, "["+string(c.code)+"]"); n != c.one {
			t.Errorf("one importer: %d %s, want %d:\n%s", n, c.code, c.one, one)
		}
		if n := strings.Count(two, "["+string(c.code)+"]"); n != c.both {
			t.Errorf("two importers: %d %s, want %d:\n%s", n, c.code, c.both, two)
		}
	}
	if n := len(reFindingCode.FindAllString(two, -1)); n != codes[0].both+codes[1].both {
		t.Errorf("%d findings, want only those counted:\n%s", n, two)
	}
	if oneReads != twoReads {
		t.Errorf("b.m read %d times for one importer, %d for two: the lets' composites are collected per package", oneReads, twoReads)
	}
}
