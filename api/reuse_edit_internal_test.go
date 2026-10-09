package canon

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"testing"
	"time"
)

// reuseEdits are edits of a: one Set, a SetCase keeping a field (judged again on the state it
// leaves, E14), and two Sets, the second applied to the state the first left (E1).
func reuseEdits() map[string][]Op {
	return map[string][]Op{
		"set":     {Set("a:uses.first.count", Int(4))},
		"setCase": {SetCase("a:uses.first.move", "run", nil)},
		"two":     {Set("a:uses.second.count", Int(5)), Set("a:uses.first.count", Int(6))},
	}
}

// API.md E17a, E1, E14, E18: an edit whose scope the kept every-package analysis serves gives what
// the analysis of its scope gives, and analyses its states again over the scope alone.
func TestEditScopeFromWholeAnalysis(t *testing.T) {
	ctx := context.Background()
	edits := reuseEdits()
	for _, name := range slices.Sorted(maps.Keys(edits)) {
		ops := edits[name]
		t.Run(name, func(t *testing.T) {
			want, err := opened(t, reuseLaw()).Edit(ctx, Edit{Ops: ops, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			kept := opened(t, reuseLaw())
			keepWhole(t, kept)
			got, err := kept.Edit(ctx, Edit{Ops: ops, DryRun: true})
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("E17a: with every package's analysis kept: %+v, %v\nwant %+v", got, err, want)
			}
			p, l := logged(t, newWriteFS(reuseLaw(), nil))
			keepWhole(t, p)
			before := l.count()
			if _, err := p.Edit(ctx, Edit{Ops: ops, DryRun: true}); err != nil {
				t.Fatal(err)
			}
			if l.loaded(before, "u/u.canon") || l.count() == before {
				t.Errorf("E17a: the edit analysed u, which it neither names nor affects, or nothing: %d checks", l.count()-before)
			}
		})
	}
}

// API.md E17a, S5: the kept every-package analysis stands for an edit's scope only where only
// values are read from it: a Set analyses its scope, then re-checks, when the project holds an
// error, when its base is not current, under an active layer (E1901), and for a Rename.
func TestEditScopeWholeAnalysisGates(t *testing.T) {
	ctx := context.Background()
	broken := reuseLaw()
	broken["/law/u/u.canon"] = []byte("/// U.\npackage u\n\n/// Broken.\nlet k: Int = \"s\"\n")
	loads := reuseLaw()
	loads["/law/u/u.canon"] = []byte("/// U.\npackage u\n\n/// Ks.\nlet ks: [Int] = load(\"k.json\")\n")
	loads["/law/u/k.json"] = []byte("[1]\n")
	layered := reuseLaw()
	layered["/law/a/live.layer.canon"] = []byte("package a\nlayer live\n\namend uses.first {\n  count: 9\n}\n")
	cases := []struct {
		name  string
		files map[string][]byte
		op    Op
		old   bool // the edit's base is a revision before u/k.json changed
		layer string
		want  int // checks: the scope's, then the re-check's
	}{
		{"served", reuseLaw(), Set("a:uses.first.count", Int(4)), false, "", 1},
		{"error", broken, Set("a:uses.first.count", Int(4)), false, "", 2},
		{"base", loads, Set("a:uses.first.count", Int(4)), true, "", 2},
		{"rename", reuseLaw(), Rename("a:uses.first", Key("third")), false, "", 2},
		{"layers", layered, Set("a:uses.second.count", Int(4)), false, "live", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := newWriteFS(maps.Clone(c.files), nil)
			var layers []string
			if c.layer != "" {
				layers = []string{c.layer}
			}
			p, l := logged(t, fsys, layers...)
			base := p.Revision()
			if c.old {
				keepWhole(t, p)
				if err := fsys.WriteFile("/law/u/k.json", []byte("[2]\n")); err != nil {
					t.Fatal(err)
				}
			} else {
				base = ""
			}
			keepWhole(t, p)
			before := l.count()
			res, err := p.Edit(ctx, Edit{Base: base, Ops: []Op{c.op}, DryRun: true, AllowErrors: true})
			if err != nil || len(res.Changes) == 0 {
				t.Fatalf("Edit: %+v, %v", res, err)
			}
			if got := l.count() - before; got != c.want {
				t.Errorf("E17a: %d checks, want %d", got, c.want)
			}
		})
	}
}

// nextOf is the next event on events, or a failure after a while.
func nextOf(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(20 * time.Second):
		t.Fatal("no event")
	}
	return Event{}
}

// API.md W13, W15, E18: the event of an applied edit reports the findings an analysis of its
// packages gives, from the edit's re-check of them when no layer is active, else from its own.
func TestWatchEditEventReadsRecheck(t *testing.T) {
	journal := reuseLaw()
	journal["/law/.canon/journal/keep"] = nil // the commit then changes nothing else the snapshot read
	layered := maps.Clone(journal)
	layered["/law/a/a.canon"] = append(layered["/law/a/a.canon"],
		[]byte("\n/// A config.\nrecord Cfg {\n  /// Port.\n  port: Int\n}\n\n/// The config.\nlet cfg: Cfg = { port: 1 }\n")...)
	layered["/law/a/live.layer.canon"] = []byte("package a\nlayer live\n\namend cfg {\n  port: 3\n}\n")
	cases := []struct {
		name   string
		files  map[string][]byte
		layers []string
		want   int // checks after the watch started: the edit's scope, its re-check, the event's
	}{
		{"recheck", journal, nil, 1},
		{"layers", layered, []string{"live"}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := newWriteFS(c.files, nil)
			p, l := logged(t, fsys, c.layers...)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			events := make(chan Event, 8)
			if err := p.Watch(ctx, func(ev Event) { events <- ev }); err != nil {
				t.Fatal(err)
			}
			seeded := l.count()
			if res, err := p.Edit(ctx, Edit{Ops: []Op{Set("a:uses.first.count", Int(4))}}); err != nil || !res.Applied {
				t.Fatalf("Edit: %+v, %v", res, err)
			}
			ev := nextOf(t, events)
			if ev.Cause != CauseEdit || ev.Err != nil || len(ev.Packages) == 0 {
				t.Fatalf("W15: event %+v", ev)
			}
			if got := l.count() - seeded; got != c.want {
				t.Errorf("W15: %d checks for the edit and its event, want %d", got, c.want)
			}
			fresh, err := Open("/law", Options{FS: fsys, Layers: c.layers})
			if err != nil {
				t.Fatal(err)
			}
			defer fresh.Close()
			want, err := fresh.Check(ctx, ev.Packages...)
			if err != nil || !reflect.DeepEqual(ev.Findings, want.Findings) || !reflect.DeepEqual(ev.Summary, want.Summary) {
				t.Errorf("W13: event findings %+v %+v, a fresh check %+v, %v", ev.Findings, ev.Summary, want, err)
			}
		})
	}
}
