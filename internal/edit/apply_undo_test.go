package edit_test

import (
	"context"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

// written is fsys with a plan's changes made: the files it writes, renames and deletes, and the
// directories it removes; a directory a file left stays, as on disk, unless the plan removes it (N6).
func written(fsys mapFS, plan *edit.Plan) mapFS {
	out := mapFS{}
	for name, f := range fsys { //canon:unordered copies a map
		out[name] = &fstest.MapFile{Data: f.Data, Mode: f.Mode}
	}
	for _, ch := range plan.Changes {
		switch ch.Kind {
		case edit.ChangeRenamed:
			leave(out, "law/"+ch.OldPath)
			out["law/"+ch.Path] = file(string(ch.After))
		case edit.ChangeDeleted, edit.ChangeRemovedDir:
			leave(out, "law/"+ch.Path)
		case edit.ChangeModified, edit.ChangeCreated:
			out["law/"+ch.Path] = file(string(ch.After))
		}
	}
	for name, f := range out { //canon:unordered drops entries one by one
		if f.Mode.IsDir() && slices.ContainsFunc(slices.Collect(maps.Keys(out)), func(n string) bool { return strings.HasPrefix(n, name+"/") }) {
			delete(out, name) // a name below it holds it
		}
	}
	return out
}

// leave removes name from fsys and keeps its directory, which a map file system holds only
// while a name lies below it.
func leave(fsys mapFS, name string) {
	delete(fsys, name)
	fsys[path.Dir(name)] = &fstest.MapFile{Mode: fs.ModeDir}
}

// values is the canonical text of every let of the analyzed packages, by name; unordered, a
// table's entries in key order: a recreated entry file is placed by its path (API.md E22).
func values(s session, unordered bool) map[string]string {
	out := map[string]string{}
	for _, pkg := range s.a.Program().Packages {
		if s.a.Bag(pkg.Path) == nil {
			continue
		}
		for _, obj := range pkg.Decls {
			if obj.Kind() != check.ObjLet {
				continue
			}
			if v, ok := s.a.Force(eval.Root{Pkg: pkg.Path, Name: obj.Name()}); ok {
				out[pkg.Path+":"+obj.Name()] = textOf(v, unordered)
			}
		}
	}
	return out
}

func textOf(v value.Value, unordered bool) string {
	t, ok := v.(*value.Table)
	if !unordered || !ok {
		return v.CanonText()
	}
	var entries []string
	for _, e := range t.Entries {
		entries = append(entries, e.Ident.Key.Text()+": "+e.CanonText())
	}
	slices.Sort(entries)
	return strings.Join(entries, ", ")
}

// known is layers when fsys has a file of layer, else base: a layer with no file amends nothing.
func known(fsys mapFS, layers, base []string, layer string) []string {
	for _, f := range fsys { //canon:unordered an any-of test
		if strings.Contains(string(f.Data), "\nlayer "+layer+"\n") {
			return layers
		}
	}
	return base
}

// checkUndo is API.md E22: the plan's Undo, applied to the state the plan leaves, restores
// every value the edit changed, with the case's layers and, when the edit layer is not among
// them, with it active too (log-2026-09-29 M4 U4b-r3); a Retire has no inverse (E23).
func checkUndo(t *testing.T, c goldenCase, ops []edit.Operation, plan *edit.Plan) {
	t.Helper()
	for _, op := range ops {
		if op.Kind == edit.OpRetire {
			return
		}
	}
	after := written(c.fsys, plan)
	s := open(t, after, c.layers, c.editLayer, c.pkgs...)
	undo, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Errorf("Undo %v does not apply (API.md E22): %v", plan.Undo, err)
		return
	}
	sets := [][]string{c.layers}
	if c.editLayer != "" && !slices.Contains(c.layers, c.editLayer) {
		sets = append(sets, append(slices.Clone(c.layers), c.editLayer))
	}
	for _, layers := range sets {
		before := values(open(t, c.fsys, known(c.fsys, layers, c.layers, c.editLayer), c.editLayer, c.pkgs...), c.undoUnordered)
		back := written(after, undo)
		restored := values(open(t, back, known(back, layers, c.layers, c.editLayer), c.editLayer, c.pkgs...), c.undoUnordered)
		for _, name := range slices.Sorted(maps.Keys(before)) {
			if v := before[name]; restored[name] != v {
				t.Errorf("with layers %v, Undo leaves %s = %s, want %s (API.md E22)", layers, name, restored[name], v)
			}
		}
	}
}
