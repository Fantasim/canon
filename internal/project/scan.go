package project

import (
	"cmp"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Scan lists every .canon file under dir, project.canon at its top excepted, skipping the
// directories whose name starts with "." (API.md O2): project-relative paths, in byte order.
func Scan(fsys FS, dir string) ([]string, error) {
	var out []string
	if err := scanDir(fsys, dir, "", &out); err != nil {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
}

// scanDir lists rel into out, sorting the listing, whose order is not trusted (API.md §2.2).
func scanDir(fsys FS, root, rel string, out *[]string) error {
	entries, err := fsys.ReadDir(path.Join(root, rel))
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	for _, e := range entries {
		name := path.Join(rel, e.Name())
		switch {
		case e.IsDir() && !strings.HasPrefix(e.Name(), hiddenPrefix):
			if err := scanDir(fsys, root, name, out); err != nil {
				return err
			}
		case IsSource(name, e):
			*out = append(*out, name)
		}
	}
	return nil
}

// IsSource reports the entry e, at the project-relative name, as a source the scan reads: a
// .canon file other than the top project.canon (API.md O2).
func IsSource(name string, e fs.DirEntry) bool {
	return !e.IsDir() && path.Ext(name) == SourceExt && name != FileName
}
