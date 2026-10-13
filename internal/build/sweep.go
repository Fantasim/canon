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

// deadStage reports name a stage of the file base another run left: its staging name with a
// token not this run's, or the temporary file the OS WriteFile wrote that name through.
func (r *run) deadStage(name, base string) bool {
	own := tempPrefix + base + tempSep + r.writes().token + tempSuffix
	if stagedOf(name, base) {
		return name != own && name != tempPrefix+base+tempSuffix // 0.1.5's name is the set-aside sweeps' only
	}
	inner, ok := strings.CutPrefix(name, tempPrefix)
	if !ok || len(inner) < len(tempSep)+tokenLen+len(tempSuffix) {
		return false
	}
	inner = inner[:len(inner)-len(tempSep)-tokenLen-len(tempSuffix)]
	return stagedOf(name, inner) && stagedOf(inner, base) && inner != tempPrefix+base+tempSuffix
}

// stageName reports a staging or set-aside name, never a file an emit writes.
func stageName(base string) bool {
	return strings.HasPrefix(base, tempPrefix) && strings.HasSuffix(base, tempSuffix)
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
