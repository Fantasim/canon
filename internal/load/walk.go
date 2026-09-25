package load

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fantasim/canonlang/internal/project"
)

// matchFile is one file a load.dir glob selected: its display form and resolved path (WIRE.md §2.3).
type matchFile struct {
	Display string
	Abs     string
}

// globMatches is base's matches: itself with no glob, else rest walked under base's resolved directory, one per resolved file, in path order; base's own link is bounds-checked first, the same rule a walk entry gets.
func (l *Loader) globMatches(base project.Path, rest string, req Request) ([]matchFile, error) {
	bounds := l.walkBounds()
	real, matched, err := l.resolveBase(base, bounds, req)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}
	if rest == "" {
		return l.literalMatch(base, real)
	}
	info, err := l.FS.Stat(real)
	switch {
	case err != nil:
		return nil, err
	case !info.IsDir():
		return nil, errNotDir
	}
	w := l.newWalker(base, bounds, req)
	hits, err := w.descend(real, func() ([]hit, error) {
		return w.dirSegs("", real, mergeStars(strings.Split(rest, sepStr)))
	})
	if err != nil {
		return nil, err
	}
	hits = firstByReal(hits)
	out := make([]matchFile, len(hits))
	for i, h := range hits {
		out[i] = matchFile{Display: path.Join(base.Display, h.rel), Abs: h.real}
	}
	return out, nil
}

// literalMatch is a glob with no magic character: base itself at its resolved path real, or nothing for a directory (WIRE.md §6.5).
func (l *Loader) literalMatch(base project.Path, real string) ([]matchFile, error) {
	info, err := l.FS.Stat(real)
	switch {
	case err != nil:
		return nil, err
	case info.IsDir():
		return nil, nil
	}
	return []matchFile{{Display: base.Display, Abs: real}}, nil
}

// mergeStars collapses every run of consecutive "**" into one, so it walks exactly once.
func mergeStars(segs []string) []string {
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		if s == doubleStar && len(out) > 0 && out[len(out)-1] == doubleStar {
			continue
		}
		out = append(out, s)
	}
	return out
}

// firstByReal is hits in ascending byte order of their relative path, only the first of each
// resolved file kept: a link and its target's own directory reach one file.
func firstByReal(hits []hit) []hit {
	slices.SortFunc(hits, func(a, b hit) int { return strings.Compare(a.rel, b.rel) })
	seen := map[string]bool{}
	out := hits[:0]
	for _, h := range hits {
		if !seen[h.real] {
			seen[h.real] = true
			out = append(out, h)
		}
	}
	return out
}

// dirSegs is every match of segs in the directory rel, resolved at real (WIRE.md §6.5).
func (w *walker) dirSegs(rel, real string, segs []string) ([]hit, error) {
	if segs[0] == doubleStar {
		return w.doubleStar(rel, real, segs[1:])
	}
	entries, err := w.l.readDir(real)
	if err != nil {
		return nil, err
	}
	var out []hit
	for _, e := range entries {
		hits, err := w.entry(rel, real, e, segs)
		if err != nil {
			return nil, err
		}
		out = append(out, hits...)
	}
	return out, nil
}

// entry is entry e's matches of segs' first segment, joined with any deeper match under it.
func (w *walker) entry(rel, real string, e fs.DirEntry, segs []string) ([]hit, error) {
	if !segMatches(segs[0], e.Name()) {
		return nil, nil
	}
	k := w.classify(rel, real, e)
	switch {
	case !k.ok:
		return nil, nil
	case len(segs) == 1 && k.isFile:
		return []hit{{rel: path.Join(rel, e.Name()), real: k.real}}, nil
	case len(segs) == 1 || !k.isDir:
		return nil, nil
	}
	return w.descend(k.real, func() ([]hit, error) {
		return w.dirSegs(path.Join(rel, e.Name()), k.real, segs[1:])
	})
}

// doubleStar matches "**" then rest in the directory rel, resolved at real: zero directories or
// more, skipping dotdirs.
func (w *walker) doubleStar(rel, real string, rest []string) ([]hit, error) {
	if len(rest) == 0 {
		return w.allFiles(rel, real)
	}
	out, err := w.dirSegs(rel, real, rest)
	if err != nil {
		return nil, err
	}
	entries, err := w.l.readDir(real)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), dotPrefix) {
			continue
		}
		k := w.classify(rel, real, e)
		if !k.ok || !k.isDir {
			continue
		}
		deeper, err := w.descend(k.real, func() ([]hit, error) {
			return w.doubleStar(path.Join(rel, e.Name()), k.real, rest)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, deeper...)
	}
	return out, nil
}

// allFiles is every regular file in the directory rel, resolved at real, at every depth,
// skipping dotfiles: a trailing "**".
func (w *walker) allFiles(rel, real string) ([]hit, error) {
	entries, err := w.l.readDir(real)
	if err != nil {
		return nil, err
	}
	var out []hit
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), dotPrefix) {
			continue
		}
		k := w.classify(rel, real, e)
		switch {
		case !k.ok:
			continue
		case k.isFile:
			out = append(out, hit{rel: path.Join(rel, e.Name()), real: k.real})
			continue
		case !k.isDir:
			continue
		}
		sub, err := w.descend(k.real, func() ([]hit, error) {
			return w.allFiles(path.Join(rel, e.Name()), k.real)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return out, nil
}

// readDir is dir's entries, empty (not an error) when dir does not exist (WIRE.md §6.5).
func (l *Loader) readDir(dir string) ([]fs.DirEntry, error) {
	entries, err := l.FS.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return entries, err
}

// segMatches is whether name matches seg: a dotfile only for a segment itself starting with '.' (WIRE.md §6.5).
func segMatches(seg, name string) bool {
	if strings.HasPrefix(name, dotPrefix) && !strings.HasPrefix(seg, dotPrefix) {
		return false
	}
	return doublestar.MatchUnvalidated(rewriteForDoublestar(seg), name)
}
