package canon

import (
	"context"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/workspace"
)

// pkgReads is what a package read at its last re-check, by absolute name (API.md S5): files and
// links, the directories listed (its own, or a load's), and those a load listed.
type pkgReads struct {
	files    map[string]bool
	listings map[string]bool
	loads    []string
}

// readsOf sorts reads into a pkgReads.
func readsOf(reads []build.Read) *pkgReads {
	r := &pkgReads{files: map[string]bool{}, listings: map[string]bool{}}
	for _, rd := range reads {
		switch {
		case rd.Dir && !rd.Sources:
			r.listings[rd.Abs] = true
			r.loads = append(r.loads, rd.Abs)
		case rd.Dir:
			r.listings[rd.Abs] = true
		default:
			r.files[rd.Abs] = true
		}
	}
	return r
}

// loadCovers reports name a load listed, or found below what it listed.
func (r *pkgReads) loadCovers(name string) bool {
	for d := name; len(r.loads) > 0; d = project.DirOf(d) {
		if slices.Contains(r.loads, d) {
			return true
		}
		if project.DirOf(d) == d {
			return false
		}
	}
	return false
}

// touches reports a package with these reads affected by name, a relevant change (S2).
func (r *pkgReads) touches(name string) bool {
	return r.files[name] || r.listings[name] || r.listings[project.DirOf(name)] || r.loadCovers(name)
}

// learn records what each package of names read in a.
func (w *watching) learn(a *build.Analysis, names []string) {
	for _, pkg := range names {
		w.reads[pkg] = readsOf(a.Reads(pkg))
	}
}

// relevance decides which names of an event count (API.md W12, S3): those in the read set
// before or now, read by a package, or gained or lost where a load lists and read by its glob
// now or before; the loads' globs run once, and only when a name needs them.
type relevance struct {
	w       *watching
	s       *workspace.Snapshot
	units   *build.Units
	unknown bool // a package whose reads are unknown: any name may be one of its loads'
	now     []string
	globbed bool
	matched []string // the names that count as files a glob reads
}

func (w *watching) relevance(s *workspace.Snapshot, units *build.Units) *relevance {
	g := &relevance{w: w, s: s, units: units}
	if units != nil {
		g.unknown = slices.ContainsFunc(units.Units, func(u *project.Unit) bool { return w.reads[u.Name] == nil })
	}
	return g
}

// globs is every file the loads read now, globbed once.
func (g *relevance) globs() []string {
	if !g.globbed {
		g.now, g.globbed = g.w.globs(g.s, g.units), true
	}
	return g.now
}

// names is every name of c that counts, and every source gained or lost, in byte order; editor
// files, .canon/, build outputs and names no glob reads never count.
func (g *relevance) names(c workspace.Change, listed, sources map[string]string) []string {
	var out []string
	for _, n := range c.Changed {
		_, before := g.w.listed[n]
		_, now := listed[n]
		if before || now || g.w.readItself(n) || g.globbedName(n, slices.Contains(c.Relisted, n)) {
			out = append(out, n)
		}
	}
	for _, n := range symmetric(g.w.sources, sources) {
		out = append(out, n.Abs)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// globbedName reports n, where a load lists, read by a load's glob now or before, or a new
// directory holding such a file.
func (g *relevance) globbedName(n string, relisted bool) bool {
	if !g.unknown && !g.w.underLoad(n) {
		return false
	}
	if !holds(g.globs(), n, relisted) && !holds(g.w.matched, n, relisted) {
		return false
	}
	g.matched = append(g.matched, n)
	return true
}

// readItself reports name a file, stat or link a package read, not a directory it listed.
func (w *watching) readItself(name string) bool {
	//canon:unordered any package that read it will do
	for _, r := range w.reads {
		if r.files[name] && !r.listings[name] {
			return true
		}
	}
	return false
}

// underLoad reports name in or below a directory a load listed.
func (w *watching) underLoad(name string) bool {
	//canon:unordered any package that listed it will do
	for _, r := range w.reads {
		if r.loadCovers(name) {
			return true
		}
	}
	return false
}

// affected is every package names touch, and every package whose reads are unknown; in order.
func (w *watching) affected(units *build.Units, names []string) []string {
	var out []string
	for _, u := range units.Units {
		r, known := w.reads[u.Name]
		if !known || slices.ContainsFunc(names, r.touches) {
			out = append(out, u.Name)
		}
	}
	return out
}

// files is every file of c, or name of c.Changed (a stat or link included), in the read set
// before or now or a source, every file first read below a changed name (a new directory), and
// every source gained or lost, as display paths in byte order (W12, S3).
func (w *watching) files(c workspace.Change, listed, sources map[string]string, globbed []string) []string {
	var out []string
	for _, f := range c.Files {
		if _, ok := w.listed[f.Abs]; ok || named(f.Abs, listed, sources) || slices.Contains(globbed, f.Abs) {
			out = append(out, f.Display)
		}
	}
	for _, n := range c.Changed {
		if d, ok := listed[n]; ok {
			out = append(out, d)
		} else if d, ok := sources[n]; ok {
			out = append(out, d)
		}
	}
	out = append(out, w.firstBelow(c.Changed, listed)...)
	for _, f := range symmetric(w.sources, sources) {
		out = append(out, f.Display)
	}
	w.listed, w.sources = listed, sources
	slices.Sort(out)
	return slices.Compact(out)
}

func named(abs string, sets ...map[string]string) bool {
	return slices.ContainsFunc(sets, func(m map[string]string) bool { _, ok := m[abs]; return ok })
}

// firstBelow is the display path of every file of listed not in the read set before and below
// a name of changed: a file in a directory that appeared.
func (w *watching) firstBelow(changed []string, listed map[string]string) []string {
	var out []string
	for _, abs := range slices.Sorted(maps.Keys(listed)) {
		if _, before := w.listed[abs]; !before && below(abs, changed) {
			out = append(out, listed[abs])
		}
	}
	return out
}

// below reports a directory above abs among changed, a sorted list.
func below(abs string, changed []string) bool {
	for d := project.DirOf(abs); ; d = project.DirOf(d) {
		if _, found := slices.BinarySearch(changed, d); found {
			return true
		}
		if project.DirOf(d) == d {
			return false
		}
	}
}

// readSet is s's read set (S3), and the sources the project scan selects (O2), absolute name to
// display path, the files every load names included (DECISIONS 330).
func readSet(ctx context.Context, s *workspace.Snapshot) (listed, sources map[string]string) {
	listed, sources = map[string]string{}, map[string]string{}
	inputs, _ := s.Build().Inputs()
	for _, r := range inputs {
		listed[r.Abs] = r.Display
	}
	addLoads(ctx, listed, s)
	b := s.Build()
	names, _ := project.Scan(b.FS(), b.Dir())
	for _, name := range names {
		sources[project.Join(b.Dir(), name)] = name
	}
	return listed, sources
}

// addLoads adds to listed every file a load of s's project names (S3, DECISIONS 330); a failure to
// find them adds none, as a failed Inputs lists less: the event's re-check reports it.
func addLoads(ctx context.Context, listed map[string]string, s *workspace.Snapshot) {
	loads, _ := s.Loads(ctx)
	for _, r := range loads {
		listed[r.Abs] = r.Display
	}
}

// symmetric is every entry one of a and b holds and the other does not, by absolute name.
func symmetric(a, b map[string]string) []workspace.Path {
	var out []workspace.Path
	for _, pair := range [][]map[string]string{{a, b}, {b, a}} {
		for _, abs := range slices.Sorted(maps.Keys(pair[0])) {
			if _, ok := pair[1][abs]; !ok {
				out = append(out, workspace.Path{Display: pair[0][abs], Abs: abs})
			}
		}
	}
	return out
}
