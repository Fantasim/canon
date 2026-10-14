package build

import (
	"bytes"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

// clash is one E8152: the colliding output's lower-cased path, its package and the finding.
type clash struct {
	key, pkg string
	b        *diag.Builder
}

// clashes keeps outputs but those on a file another output writes, case aside, a runtime file of the same content excepted, or on a canon.lock or canon.outputs path: each a clash (WIRE.md §8.1, CODEGEN.md §2.9).
func (r *run) clashes(outputs []*output) ([]*output, []clash) {
	first := map[string]*output{}
	reserved := r.reserved()
	kept := outputs[:0:0]
	var found []clash
	for _, o := range outputs {
		key := strings.ToLower(o.Abs)
		if o.remove { // a deletion is no output: a path differing in case from an output is another file (CODEGEN.md §2.9, DECISIONS 336)
			kept = append(kept, o)
			continue
		}
		if rel, clashes := reserved[key]; clashes && !o.listing {
			found = append(found, clash{key: key, pkg: o.Package, b: diag.E8152.AtFile(o.at, rel, o.Path)})
			continue
		}
		prev, seen := first[key]
		switch {
		case !seen:
			first[key] = o
			kept = append(kept, o)
		case prev.Abs != o.Abs || !bytes.Equal(prev.Content, o.Content) || !runtimeFile(o.Abs):
			found = append(found, clash{key: key, pkg: o.Package, b: diag.E8152.AtFile(o.at, prev.Path, o.Path)})
		}
	}
	return kept, found
}

// collide is E8152 after stage E from the placed emits of the selected packages, every target, no output read: only those sharing a directory or writing into a package's are generated, in a run without error (free); a view's one file is known (DECISIONS 355).
func (r *run) collide(free bool) {
	var outputs []*output
	for _, pe := range r.sharing() {
		if pe.e.Target == ir.TargetView || free {
			outputs = append(outputs, r.emitPaths(pe.p, pe.e)...)
		}
	}
	_, found := r.clashes(outputs)
	for _, c := range found {
		c.b.Report(r.bags[c.pkg])
		r.writes().collided[c.key] = true
	}
}

// sharing is the run's placed emits whose directories another emit writes into too, or that
// write into a directory holding a source or above one, where a package keeps its canon.lock.
func (r *run) sharing() []*placedEmit {
	emits := r.placedEmits()
	count, packages := map[string]int{}, map[string]bool{}
	for _, pe := range emits {
		for _, d := range emitDirs(pe) {
			count[d]++
		}
	}
	for key := range r.reserved() { //canon:unordered a set
		packages[path.Dir(key)] = true
	}
	var out []*placedEmit
	for _, pe := range emits {
		for _, d := range emitDirs(pe) {
			if count[d] > 1 || packages[d] {
				out = append(out, pe)
				break
			}
		}
	}
	return out
}

// emitDirs are the lower-cased directories pe writes into: its own, and Go's runtime directory.
func emitDirs(pe *placedEmit) []string {
	dir := strings.ToLower(pe.dir)
	if pe.e.Target == ir.TargetGo {
		return []string{dir, project.Join(dir, ir.GoRT)}
	}
	return []string{dir}
}

// fileNames are the files e writes, by name: a view's one file and a json emit's from its layout,
// no content encoded (the data of every entry otherwise, on each check and edit re-check); the
// other targets' from their generator, which the write then shares (DECISIONS 355).
func (r *run) fileNames(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	if e.Target == ir.TargetView {
		return []ir.File{{Path: e.FileName}}, nil
	}
	if err := complete(p); err != nil {
		return nil, err
	}
	if e.Target != ir.TargetJSON {
		return r.generate(p, e)
	}
	paths, err := jsongen.Paths(p, e)
	files := make([]ir.File, len(paths))
	for i, name := range paths {
		files[i] = ir.File{Path: name}
	}
	return files, err
}
