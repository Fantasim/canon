package project

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Layout places a project and its roots on disk, --root overrides applied (WIRE.md §2.2).
type Layout struct {
	Dir   string
	names []string
	dirs  map[string]string
}

// NewLayout resolves each root of p lexically against dir, or takes its override: an absolute
// directory, or one relative to dir. An override of an undeclared root is E7003.
func NewLayout(p *Project, dir string, overrides map[string]string, bag *diag.Bag) (*Layout, bool) {
	l := &Layout{Dir: path.Clean(dir), dirs: map[string]string{}}
	for _, r := range p.Roots {
		l.names = append(l.names, r.Name)
		l.dirs[r.Name] = path.Join(l.Dir, r.Path)
	}
	ok := true
	for _, name := range slices.Sorted(maps.Keys(overrides)) {
		if _, declared := l.dirs[name]; !declared {
			diag.E7003.At(source.Span{}, name, l.names).Report(bag)
			ok = false
			continue
		}
		l.dirs[name] = l.abs(overrides[name])
	}
	return l, ok
}

// RootDirs is every declared root's resolved directory, sorted (meta/decisions/log-2026-09-24.md
// "load.dir round 2": where a followed symbolic link may point and still count as inside a root).
func (l *Layout) RootDirs() []string {
	return slices.Sorted(maps.Values(l.dirs))
}

// abs is dir itself when absolute, else dir under the project directory.
func (l *Layout) abs(dir string) string {
	if isAbsolute(dir) {
		return path.Clean(dir)
	}
	return path.Join(l.Dir, dir)
}

// Path is a resolved path: its display form (WIRE.md §2.3) and the file on disk.
type Path struct {
	Display string
	Abs     string
	Root    string
	Dir     bool
}

// Resolve resolves a path written in a file of the project-relative directory from (WIRE.md §2).
func (l *Layout) Resolve(written, from string, span source.Span, bag *diag.Bag) (Path, bool) {
	b := syntaxError(written, span)
	var p Path
	if b == nil {
		p, b = l.resolve(written, from, span)
	}
	if b != nil {
		b.Report(bag)
		return Path{}, false
	}
	return p, true
}

// resolve places a well-formed path: a rooted one may not climb above its root, an unrooted
// one, starting at from, not above the project directory.
func (l *Layout) resolve(written, from string, span source.Span) (Path, *diag.Builder) {
	body, isDir := strings.CutSuffix(written, sep)
	name, rest, rooted := cutRoot(body)
	base, prefix, start := l.Dir, "", splitRest(from)
	if rooted {
		dir, declared := l.dirs[name]
		if !declared {
			return Path{}, diag.E7003.At(span, name, l.names)
		}
		base, prefix, start = dir, rootMark+name, nil
	}
	segs, ok := normalize(start, splitRest(rest))
	switch {
	case !ok && rooted:
		return Path{}, diag.E7001.AtRoot(span, written, name)
	case !ok:
		return Path{}, diag.E7001.AtProject(span, written)
	}
	display := strings.Join(segs, sep)
	switch {
	case prefix != "" && display != "":
		display = prefix + sep + display
	case prefix != "":
		display = prefix
	case display == "":
		display = currentSeg
	}
	if isDir {
		display += sep
	}
	abs := path.Join(append([]string{base}, segs...)...)
	return Path{Display: display, Abs: abs, Root: name, Dir: isDir}, nil
}

// syntaxError is E7001 for an absolute path, a backslash or an empty segment (WIRE.md §2.1).
func syntaxError(written string, span source.Span) *diag.Builder {
	switch {
	case isAbsolute(written):
		return diag.E7001.AtAbsolute(span, written)
	case strings.Contains(written, backslash):
		return diag.E7001.AtBackslash(span, written)
	}
	body, _ := strings.CutSuffix(written, sep)
	if _, rest, rooted := cutRoot(body); rooted {
		if !strings.Contains(body, sep) {
			return nil
		}
		body = rest
	}
	if slices.Contains(strings.Split(body, sep), "") {
		return diag.E7001.AtEmpty(span, written)
	}
	return nil
}

// cutRoot splits "@name/rest" into name and rest; "@name" alone is the root itself.
func cutRoot(p string) (name, rest string, rooted bool) {
	after, rooted := strings.CutPrefix(p, rootMark)
	if !rooted {
		return "", p, false
	}
	name, rest, _ = strings.Cut(after, sep)
	return name, rest, true
}

func splitRest(rest string) []string {
	if rest == "" {
		return nil
	}
	return strings.Split(rest, sep)
}

// normalize applies segs to base lexically; climbing above base fails (WIRE.md §2.2).
func normalize(base, segs []string) ([]string, bool) {
	out := slices.Clone(base)
	for _, s := range segs {
		switch s {
		case currentSeg:
		case parentSeg:
			if len(out) == 0 {
				return nil, false
			}
			out = out[:len(out)-1]
		default:
			out = append(out, s)
		}
	}
	return out, true
}
