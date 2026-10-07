package build

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// sumLine is a file, glob or list line: its path or pattern and its SHA-256 in lower-case hex (WIRE.md §10).
type sumLine struct {
	key, sum string
}

// inputs is what a run's loads and asset checks read, taken when its command ends (WIRE.md §10).
type inputs struct {
	loaded map[string]loadedFile // by name
	globs  []globMatch
	lists  []listed
}

// inputs takes what the run's hosts read so far: a later ViewModel's reads are not in it.
func (r *run) inputs() inputs {
	in := inputs{loaded: map[string]loadedFile{}}
	hosts := r.manifestHosts()
	for _, h := range hosts {
		if l := h.log(); l != nil {
			l.loaded(in.loaded)
			in.globs = append(in.globs, l.usedGlobs()...)
		}
	}
	in.lists = listings(hosts)
	return in
}

// manifest is the build manifest of in, read by this run of command with the --target values targets (WIRE.md §10, API.md O7).
func (r *run) manifest(in inputs, command string, targets []ir.Target) []byte {
	var b strings.Builder
	b.WriteString(manifestHeader + lineEnd)
	writeLine(&b, kwCompiler, CompilerVersion)
	writeLine(&b, kwLanguage, r.s.proj.Canon.String())
	for _, layer := range r.p.opt.Layers {
		writeLine(&b, kwLayer, layer)
	}
	writeLine(&b, kwLang, cmp.Or(r.p.opt.Lang, r.s.proj.SourceLanguage()))
	writeLine(&b, kwCommand, command)
	for _, word := range targetLines(targets) {
		writeLine(&b, kwTarget, word)
	}
	for _, name := range slices.Sorted(slices.Values(r.selectedNames())) {
		writeLine(&b, kwPackage, name)
	}
	for _, root := range r.rootLines() {
		writeLine(&b, kwRoot, root)
	}
	writeSums(&b, kwFile, r.fileLines(in.loaded))
	writeSums(&b, kwGlob, globLines(in.globs))
	writeSums(&b, kwList, listLines(in.lists))
	return []byte(b.String())
}

// rootLines is each declared root by name and its directory relative to the project's,
// project.local.canon and --root applied, an absent one marked (API.md O7, DECISIONS 332).
func (r *run) rootLines() []string {
	return rootLinesOf(r.p.dir, r.s.proj, r.s.layout)
}

// rootLinesOf is rootLines for the project proj at dir placed by layout.
func rootLinesOf(dir string, proj *project.Project, layout *project.Layout) []string {
	bag := diag.NewBag(nil, "")
	out := make([]string, 0, len(proj.Roots))
	for _, root := range proj.Roots {
		placed, ok := layout.Resolve(rootMark+root.Name, "", source.Span{}, bag)
		if !ok {
			continue
		}
		line := root.Name + lineSep + relativeDir(dir, placed.Abs)
		if layout.Absent(root.Name) {
			line += lineSep + kwAbsent
		}
		out = append(out, line)
	}
	slices.Sort(out)
	return out
}

// relativeDir is dir relative to base, '/'-separated; where none exists, "sha256:" and the hash of dir, never a path (log M4 B5-r).
func relativeDir(base, dir string) string {
	rel, err := filepath.Rel(filepath.FromSlash(base), filepath.FromSlash(dir))
	if err != nil {
		return hashPrefix + hashOf([]string{filepath.ToSlash(dir)}, "")
	}
	return filepath.ToSlash(rel)
}

// hashOf is the lower-case hex SHA-256 of lines, each followed by end.
func hashOf(lines []string, end string) string {
	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(l + end))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeLine writes one line: its keyword, then text.
func writeLine(b *strings.Builder, keyword, text string) {
	b.WriteString(keyword + lineSep + text + lineEnd)
}

// writeSums writes lines sorted by path or pattern bytes, each once: keyword, SHA-256, path.
func writeSums(b *strings.Builder, keyword string, lines []sumLine) {
	slices.SortFunc(lines, func(x, y sumLine) int { return cmp.Or(cmp.Compare(x.key, y.key), cmp.Compare(x.sum, y.sum)) })
	for _, l := range slices.Compact(lines) {
		writeLine(b, keyword, l.sum+lineSep+l.key)
	}
}

// targetLines is the words of targets, sorted, each once; every target's when none is selected.
func targetLines(targets []ir.Target) []string {
	words := make([]string, 0, len(targetWords))
	for t, word := range targetWords {
		if len(targets) == 0 || slices.Contains(targets, ir.Target(t)) {
			words = append(words, word)
		}
	}
	slices.Sort(words)
	return slices.Compact(words)
}

// manifestHosts is every host whose loads and asset checks fed the run: its own, then the drivers-only program's (VIEWMODEL.md 12.3).
func (r *run) manifestHosts() []*evalHost {
	var out []*evalHost
	if r.host != nil {
		out = append(out, r.host)
	}
	if r.wideReads != nil {
		out = append(out, r.wideReads.host)
	}
	return out
}

// fileLines is every file read: project.canon, sources and locks, then the loads' files by first site (WIRE.md §2.3).
func (r *run) fileLines(loaded map[string]loadedFile) []sumLine {
	out := make([]sumLine, 0, len(r.s.sums)+len(loaded))
	own := make(map[string]bool, len(r.s.sums))
	for _, f := range r.s.sums {
		if f.Path == project.LocalFileName { // its effect is the root lines (WIRE.md §10)
			continue
		}
		own[project.Join(r.p.dir, f.Path)] = true
		out = append(out, sumLine{key: f.Path, sum: hex.EncodeToString(f.Sum[:])})
	}
	//canon:unordered writeSums sorts the lines
	for abs, f := range loaded {
		if !own[abs] {
			out = append(out, sumLine{key: f.display, sum: hex.EncodeToString(f.sum[:])})
		}
	}
	return out
}

// globLines is every load.dir the run used: its pattern, and the SHA-256 of its matches in match order (WIRE.md §10).
func globLines(globs []globMatch) []sumLine {
	out := make([]sumLine, len(globs))
	for i, g := range globs {
		out[i] = sumLine{key: g.pattern, sum: hashOf(g.matched, lineEnd)}
	}
	return out
}

// listLines is each listing's line: the SHA-256 of its names sorted, one per line (WIRE.md §10, TYP-21).
func listLines(lists []listed) []sumLine {
	out := make([]sumLine, len(lists))
	for i, l := range lists {
		out[i] = sumLine{key: l.display, sum: hashOf(listedNames(l.names), lineEnd)}
	}
	return out
}
