package build

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/project"
)

// sweepDead removes, under the run's exclusive locks, the stages dead runs left beside the files
// it wrote or removed, whose every writer takes those locks; not beside a runtime file a package
// it holds no lock on may also write, whose build may be live (DECISIONS 352).
func (r *run) sweepDead(opt BuildOptions, outputs []*output) {
	fsys, ok := r.p.fs.(WriteFS)
	if !ok || !r.locking(opt) {
		return
	}
	for _, o := range outputs {
		if o.under != "" || o.consumer != "" || stageName(path.Base(o.Abs)) || runtimeFile(o.Abs) && r.unheldWriter(o.Abs) {
			continue
		}
		dir := project.DirOf(o.Abs)
		names, err := r.writes().listing(r.p.fs, dir)
		if err != nil {
			continue // a directory that cannot be listed keeps its stages: the next build sweeps them
		}
		for _, n := range names {
			if r.deadStage(n, path.Base(o.Abs)) {
				_ = fsys.Remove(project.Join(dir, n)) // a stage that stays is swept by the next build
			}
		}
	}
}

// unheldWriter reports a package the run holds no lock on whose emits may write into abs's
// directory: its build may be live.
func (r *run) unheldWriter(abs string) bool {
	pkgs, _ := r.emittersInto(map[string]bool{strings.ToLower(project.DirOf(abs)): true})
	for _, p := range pkgs {
		if !r.loads(p) {
			return true
		}
	}
	return false
}
