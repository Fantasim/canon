package build

import (
	"cmp"
	"crypto/sha256"
	"slices"

	"github.com/fantasim/canonlang/internal/project"
)

type sha256Sum = [sha256.Size]byte

// siteKey orders load sites by package, file, then offset, the zero key last (WIRE.md §2.3).
type siteKey struct {
	pkg, file string
	at        int
}

// before reports k ahead of o in package, then source order.
func (k siteKey) before(o siteKey) bool {
	switch {
	case k.file == "":
		return false
	case o.file == "":
		return true
	}
	return cmp.Or(cmp.Compare(k.pkg, o.pkg), cmp.Compare(k.file, o.file), cmp.Compare(k.at, o.at)) < 0
}

// loadedFile is a file the run's loads kept: the SHA-256 of its bytes, and the display its first site gave it.
type loadedFile struct {
	sum     sha256Sum
	site    siteKey
	display string
}

// globMatch is a load.dir the run used: its pattern's display and its matches' displays, in match order.
type globMatch struct {
	pattern string
	matched []string
}

// trackLoad is track for the load at site, whose files keep the display of their first site (WIRE.md §2.3).
func (h *evalHost) trackLoad(site loadSite) func() {
	done := h.track(site.pkg)
	l := h.log()
	if l == nil {
		return done
	}
	next := siteKey{pkg: site.pkg, file: site.file.Src.Path, at: int(site.span.Start)}
	l.mu.Lock()
	prev := l.site
	l.site = next
	l.mu.Unlock()
	return func() {
		l.mu.Lock()
		l.site = prev
		l.mu.Unlock()
		done()
	}
}

// keep notes a file handed to the file set: its first bytes' SHA-256, its first site's resolved display (WIRE.md §10).
func (l *readLog) keep(display, abs string, data []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	had, ok := l.kept[abs]
	switch {
	case !ok:
		l.kept[abs] = loadedFile{sum: sha256.Sum256(data), site: l.site, display: display}
	case l.site.before(had.site):
		l.kept[abs] = loadedFile{sum: had.sum, site: l.site, display: display}
	}
}

// globbed keeps a load.dir's matches as the run used them (load.Loader.Globbed).
func (l *readLog) globbed(pattern string, matched []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.globs = append(l.globs, globMatch{pattern: pattern, matched: matched})
}

// loaded merges l's kept files into into, the first site's display winning (WIRE.md §2.3).
func (l *readLog) loaded(into map[string]loadedFile) {
	l.mu.Lock()
	defer l.mu.Unlock()
	//canon:unordered a merge by name, which the manifest sorts
	for abs, f := range l.kept {
		if had, ok := into[abs]; !ok || f.site.before(had.site) {
			into[abs] = f
		}
	}
}

// usedGlobs is every load.dir match l's loads used.
func (l *readLog) usedGlobs() []globMatch {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.globs)
}

// listRef is one way an asset check reached a listed directory: the asset root, the directory
// it was written in, and the display that gave the directory.
type listRef struct {
	root, from, display string
}

// listAt is list(at.Abs), noting at's display as the asset root root written in from reached it (WIRE.md §2.3).
func (a *assets) listAt(at project.Path, root, from string) dirListing {
	ref := listRef{root: root, from: from, display: at.Display}
	if !slices.Contains(a.shown[at.Abs], ref) {
		if a.shown == nil {
			a.shown = map[string][]listRef{}
		}
		a.shown[at.Abs] = append(a.shown[at.Abs], ref)
	}
	return a.list(at.Abs)
}

// listedNames is the names of a listing sorted, each folder's with a trailing '/' (WIRE.md §2.3, §10).
func listedNames(l dirListing) []string {
	names := make([]string, 0, len(l.files)+len(l.subs))
	//canon:unordered sorted below
	for name := range l.files {
		names = append(names, name)
	}
	//canon:unordered sorted below
	for name := range l.subs {
		names = append(names, name+pathSep)
	}
	slices.Sort(names)
	return names
}
