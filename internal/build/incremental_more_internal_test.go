package build

import (
	"path"
	"strings"
	"testing"
)

const (
	layersCase = "testdata/incremental/layers.txtar"
	loadsCase  = "testdata/incremental/loads.txtar"
	stableCase = "testdata/incremental/stable.txtar"
	probeEdits = 8
)

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 M4 U8-r): edits of the JSON files loads read.
func TestIncrementalLoadedJSON(t *testing.T) {
	z := archiveAnalyzer(t, loadsCase)
	files := []string{path.Join(archiveRoot, "resource/cfg.json"), path.Join(archiveRoot, "a/e1.canon"), path.Join(archiveRoot, "a/e2.canon")}
	e := newEditor(t, z, files, files[1:])
	e.run(t, probeEdits)
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, EVALUATION.md §9.3 (log-2026-09-29 M4 U8-r): edits with a layer active.
func TestIncrementalLayers(t *testing.T) {
	z := archiveAnalyzer(t, layersCase)
	z.opt.Layers = []string{"dev"}
	files := []string{path.Join(archiveRoot, "a/dev.canon"), path.Join(archiveRoot, "a/e1.canon"), path.Join(archiveRoot, "a/e2.canon")}
	e := newEditor(t, z, files, files[1:])
	e.run(t, probeEdits)
}

// structuralStep changes a project's files on disk, not in an overlay.
type structuralStep struct {
	name string
	do   func(m roFS)
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, API.md E20 (log-2026-09-29 M4 U8-r): structural edits, as cold.
func TestIncrementalStructural(t *testing.T) {
	z := archiveAnalyzer(t, stableCase)
	m := z.fs.base.(roFS)
	file := func(name string) string { return strings.TrimPrefix(path.Join(archiveRoot, name), "/") }
	edit := func(name, old, new string) func(roFS) {
		return func(m roFS) { m[file(name)] = srcFile(strings.Replace(string(m[file(name)].Data), old, new, 1)) }
	}
	added := "package a\n\nentry statuses.late {\n  code: 3\n}\n"
	for _, st := range []structuralStep{
		{"weight edited", edit("a/s/open.canon", "weight: 3", "weight: 4")},
		{"entry file added", func(m roFS) { m[file("a/s/late.canon")] = srcFile(added) }},
		{"added entry edited", edit("a/s/late.canon", "code: 3", "code: 7")},
		{"weight edited again", edit("a/s/done.canon", "weight: 5", "weight: 6")},
		{"lock line removed", edit("a/canon.lock", "table  a.statuses  done\n", "")},
		{"lock line put back", edit("a/canon.lock", "table  a.statuses  open\n", "table  a.statuses  done\ntable  a.statuses  open\n")},
		{"project.canon edited", edit("project.canon", "\"0.1\"\n", "\"0.1\"\n  budget: 100000\n")},
		{"weight edited after", edit("a/s/open.canon", "weight: 4", "weight: 2")},
		{"entry file removed", func(m roFS) { delete(m, file("a/s/late.canon")) }},
		{"weight edited last", edit("a/s/open.canon", "weight: 2", "weight: 9")},
	} {
		st.do(m)
		warm, cold := z.pair(t)
		same(t, st.name, warm, cold)
	}
}
