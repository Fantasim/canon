package build

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval"
)

// Covers reports that a gives every package's values (log-2026-09-29 M4 P14-r, DECISIONS 244):
// all, every package's analysis where the packages a leaves out read what they read on a's
// snapshot and none of a's, spent less than the budget, and so do its steps on those plus a's.
func Covers(all, a *Analysis) bool {
	if len(all.r.selected) != len(all.r.s.units) || all.r.opt.Budget != a.r.opt.Budget {
		return false
	}
	var total, left int64
	for _, c := range all.charges() {
		total += c.Steps
		if !a.r.selects(c.Pkg) {
			left += c.Steps
		}
	}
	for _, c := range a.charges() {
		left += c.Steps
	}
	budget := a.r.opt.Budget
	if budget <= 0 {
		budget = eval.DefaultBudget
	}
	return total < budget && left < budget
}

// charges is every root's steps a's run spent, phase 2's folds included.
func (a *Analysis) charges() []eval.Charge {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.r.ev.Charged()
}

// Readers are the packages owning one of files or a listing of dirs by their real paths, sorted,
// and the other names they read each of files by (API.md S5, E17; log-2026-09-29 M4 P14-r2,
// P14-r3); what no package asked for counts for every one.
func (a *Analysis) Readers(files, dirs []string) (pkgs []string, aliases map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	real := newRealPaths(a.r.p.fs)
	wantFiles, wantDirs := map[string]string{}, map[string]bool{}
	for _, f := range files {
		wantFiles[real.file(f)] = f
	}
	for _, d := range dirs {
		wantDirs[real.dir(d)] = true
	}
	aliases = map[string]string{}
	hit := func(rd Read) bool {
		if rd.Dir {
			return wantDirs[real.dir(rd.Abs)]
		}
		f, ok := wantFiles[real.file(rd.Abs)]
		if ok && f != rd.Abs && !rd.Link {
			aliases[rd.Abs] = f
		}
		return ok
	}
	l := a.r.host.log()
	every := l != nil && l.touches("", hit)
	for _, u := range a.r.s.units {
		own := l != nil && l.touches(u.Name, hit)
		if every || own || slices.ContainsFunc(a.r.p.unitReads(u), hit) {
			pkgs = append(pkgs, u.Name)
		}
	}
	slices.Sort(pkgs)
	return pkgs, aliases
}

// touches reports a name pkg consulted that hit takes, asking hit of every one.
func (l *readLog) touches(pkg string, hit func(Read) bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	found := false
	//canon:unordered every name is asked; the answer is whether one hit
	for t := range l.by[pkg] {
		if hit(Read{Abs: t.abs, Dir: t.dir, Link: t.link}) {
			found = true
		}
	}
	return found
}

// RealPaths are names resolved through the links of the file system a's snapshot reads, as
// Readers compares them (API.md S12; log-2026-09-29 M4 P14-r4).
func (a *Analysis) RealPaths(names ...string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	real := newRealPaths(a.r.p.fs)
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = real.file(n)
	}
	return out
}
