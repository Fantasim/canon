package build

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

// textFiles is an `emit text` copy's files: each `@text` file, then `.canon-text` (CODEGEN.md §2.9).
func (r *run) textFiles(p *ir.Package, _ *ir.Emit) ([]ir.File, error) {
	i := slices.IndexFunc(r.cps, func(cp *check.Package) bool { return cp.Path == p.Name })
	if i < 0 {
		return nil, internal(fmt.Errorf(fmtUnchecked, p.Name))
	}
	files, err := ir.TextFiles(r.cps[i], p)
	if err != nil {
		return nil, internal(err)
	}
	return append(files, ir.File{Path: textListing, Content: textListingOf(p.Dir, files)}), nil
}

// textListingOf is a `.canon-text`: the marker naming dir, then the file names in byte order (CODEGEN.md §2.9).
func textListingOf(dir string, files []ir.File) []byte {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Path
	}
	slices.Sort(names)
	var b strings.Builder
	fmt.Fprintf(&b, textMarkerFormat, dir)
	for _, n := range names {
		b.WriteString(n + lineEnd)
	}
	return []byte(b.String())
}

// owned reports an output canon may overwrite: marked, or named by its directory's marked `.canon-text` (CODEGEN.md §2.4, §2.9); a listing that cannot be read is an error.
func (r *run) owned(o *output, old []byte) (bool, error) {
	if o.Target != ir.TargetText {
		return marked(o.Output, old), nil
	}
	name := path.Base(o.Path)
	if name == textListing {
		return firstLineMatches(old, textMarker), nil
	}
	listing, err := r.p.fs.ReadFile(project.Join(project.DirOf(o.Abs), textListing))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, displayError(path.Join(path.Dir(o.Path), textListing), err)
	}
	return firstLineMatches(listing, textMarker) && slices.Contains(textNames(listing), name), nil
}

// textNames are the file names a `.canon-text` lists after its marker line.
func textNames(listing []byte) []string {
	lines := strings.Split(string(listing), lineEnd)
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, carriageReturn)
	}
	return lines[1:]
}
