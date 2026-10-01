package edit_test

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// diskProject writes files under a new directory and returns it.
func diskProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	for name, text := range files { //canon:unordered each file is written alone
		p := filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), filePerm); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// diskRead are the files under dir, '/'-separated and relative to it, with their bytes.
func diskRead(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	root := filepath.FromSlash(dir)
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// onDisk is the project in dir, analyzed: its session and the site a commit writes through.
func onDisk(t *testing.T, dir string) (session, edit.Site) {
	t.Helper()
	p, err := build.Open(build.OS(), dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"d"})
	if err != nil {
		t.Fatal(err)
	}
	s := session{p: p, a: a, snap: edit.NewSnapshot(a), env: edit.Env{Project: p, Host: hostOf}}
	return s, edit.Site{FS: build.OS(), Layout: p, Self: edit.Process{Host: testHost, PID: testPID}}
}

// commitOnDisk applies ops to the project in dir and commits the plan (API.md N6, N8): what the
// disk then holds is what the plan's overlay held, file by file.
func commitOnDisk(t *testing.T, name, dir string, ops []edit.Operation) (*edit.Plan, bool) {
	t.Helper()
	s, site := onDisk(t, dir)
	want := diskRead(t, dir)
	plan, over, err := edit.PlanOverlay(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err == nil {
		err = edit.Commit(context.Background(), site, testRev, plan.Changes)
	}
	if err != nil {
		t.Errorf("%s: %+v: %v", name, ops, err)
		return nil, false
	}
	prefix := strings.TrimSuffix(dir, "/") + "/"
	for abs, data := range over { //canon:unordered fills a map by name
		if rel := strings.TrimPrefix(abs, prefix); data == nil {
			delete(want, rel)
		} else {
			want[rel] = string(data)
		}
	}
	if got := diskRead(t, dir); !maps.Equal(got, want) {
		t.Errorf("%s: %+v: the disk holds %q, the overlay %q (changes %+v)", name, ops, slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)), plan.Changes)
	}
	return plan, true
}

func filesProject(t *testing.T) string {
	return diskProject(t, map[string]string{"project.canon": projectCanon, "d/d.canon": filesSrc,
		"d/q/kill/slay.canon":      "package d\n\n/// Slay.\nentry quests.slay { goal: kill, target: \"wolf\" }\n",
		"d/q/kill/hunt.canon":      "package d\n\n/// Hunt.\nentry quests.hunt { goal: kill, target: \"deer\" }\n",
		"d/q/collect/gather.canon": "package d\n\n/// Gather.\nentry quests.gather { goal: collect, target: 10 }\n",
	})
}

// API.md N3, N6, N8, E11, E22 (log-2026-10-01 M4.1 ruling: Changes are each path's net effect):
// renames, removals and additions of entry files in every order, committed on disk: the disk
// holds what the overlay held, every value comes back after the Undo, and its Undo commits too.
func TestCommitNetFileChanges(t *testing.T) {
	ren := func(p, k string) edit.Operation {
		return edit.Operation{Kind: edit.OpRename, Path: p, Key: edit.Key(k)}
	}
	rm := func(p string) edit.Operation { return edit.Operation{Kind: edit.OpRemove, Path: p} }
	add := func(k, src string) edit.Operation {
		return edit.Operation{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key(k), Value: edit.Source(src)}
	}
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"renamed, then removed", []edit.Operation{ren("quests.slay", "tmp"), rm("quests.tmp")}},
		{"created, then renamed", []edit.Operation{add("new", `{ goal: kill, target: "n" }`), ren("quests.new", "newer")}},
		{"removed, then recreated", []edit.Operation{rm("quests.slay"), add("slay", `{ goal: kill, target: "y" }`)}},
		{"removed, recreated, renamed", []edit.Operation{rm("quests.slay"), add("slay", `{ goal: collect, target: 3 }`), ren("quests.slay", "tmp")}},
		{"a chain a, b, c", []edit.Operation{ren("quests.slay", "a"), ren("quests.a", "b"), ren("quests.b", "c")}},
		{"renamed back", []edit.Operation{ren("quests.slay", "tmp"), ren("quests.tmp", "slay")}},
		{"renamed, its old key added again", []edit.Operation{ren("quests.slay", "tmp"), add("slay", `{ goal: kill, target: "z" }`)}},
		{"swapped", []edit.Operation{ren("quests.slay", "tmp"), ren("quests.hunt", "slay"), ren("quests.tmp", "hunt")}},
		{"removed, then renamed into (N3 on a freed path)", []edit.Operation{rm("quests.slay"), ren("quests.hunt", "slay")}},
		{"renamed back, then renamed into", []edit.Operation{ren("quests.slay", "tmp"), ren("quests.tmp", "slay"), ren("quests.hunt", "tmp")}},
		{"swapped through a freed name, then edited", []edit.Operation{ren("quests.slay", "tmp"), ren("quests.hunt", "slay"), ren("quests.slay", "hunt"),
			setAt("quests.gather.goal", edit.Member("collect"))}},
	} {
		threeRounds(t, c.name, c.ops)
	}
	dir := filesProject(t)
	if _, ok := commitOnDisk(t, "renamed, then removed", dir, []edit.Operation{ren("quests.slay", "tmp"), rm("quests.tmp")}); ok {
		if _, err := os.Stat(filepath.Join(filepath.FromSlash(dir), "d", "q", "kill", "tmp.canon")); err == nil {
			t.Errorf("renamed, then removed: a file is left at the new path")
		}
	}
}

// threeRounds commits ops on a new project, then their Undo, which gives every value back
// (E22), then the Undo of that Undo.
func threeRounds(t *testing.T, name string, ops []edit.Operation) {
	t.Helper()
	dir := filesProject(t)
	before, _ := onDisk(t, dir)
	start := values(before, true)
	for _, round := range []string{"request", "Undo", "Undo of the Undo"} {
		plan, ok := commitOnDisk(t, name+", "+round, dir, ops)
		if !ok {
			return
		}
		if s, _ := onDisk(t, dir); round == "Undo" && !maps.Equal(values(s, true), start) {
			t.Errorf("%s: the Undo leaves %v, want %v (E22)", name, values(s, true), start)
		}
		ops = plan.Undo
	}
}
