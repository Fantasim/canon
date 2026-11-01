package build

import (
	"io/fs"

	"github.com/fantasim/canonlang/internal/project"
)

// entryLink is a listing entry's link verdict: whether a reader takes it as a link, and its
// resolution when it does.
type entryLink struct {
	link   bool
	target string
	err    error
}

// linkOf is whether dir's entry e is a link: a LinkCandidate that is not Unlinked, judged through resolve, the reader's recorded resolver (WIRE.md §6.5, DECISIONS 419).
func linkOf(fsys project.FS, resolve func(string) (string, error), dir string, e fs.DirEntry) entryLink {
	if !project.LinkCandidate(fsys, e.Type()) {
		return entryLink{}
	}
	child := project.Join(dir, e.Name())
	target, err := resolve(child)
	if project.Unlinked(project.ResolvedAs(resolve, child, target, err), dir, e.Name()) {
		return entryLink{}
	}
	return entryLink{link: true, target: target, err: err}
}
