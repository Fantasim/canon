package project

import (
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Unit is one package of the project: the files whose package line names it (SPEC §3.2).
type Unit struct {
	Name    string
	Dir     string         // display path of its directory: its name, '/'-separated
	Files   []*syntax.File // sources, layers and translations, in path order
	Imports []string       // sorted, each once
	Layers  []string       // the names of its layer files, sorted, each once
}

// FileSum is the SHA-256 of a file as read, by display path (API.md S3).
type FileSum struct {
	Path string
	Sum  [sha256.Size]byte
}

// Reader parses the source files of the project in Dir into Set, each file's findings going to
// the bag BagOf gives for its package. A file without a package line joins no package: its
// findings go to its directory's package when one exists, else to the project's own, "".
type Reader struct {
	FS    FS
	Dir   string
	Set   *source.FileSet
	BagOf func(pkg string) *diag.Bag
	Sums  []FileSum // every file read, in reading order
}

// Parse reads and parses the files names (Scan's) and groups them by package, in name order.
func (r *Reader) Parse(ctx context.Context, names []string) ([]*Unit, error) {
	units := map[string]*Unit{}
	var orphans []*source.File
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		src, err := r.read(name)
		if err != nil {
			return nil, err
		}
		f, pkg := r.parse(src)
		if pkg == "" {
			orphans = append(orphans, src)
			continue
		}
		u := units[pkg]
		if u == nil {
			u = &Unit{Name: pkg, Dir: strings.ReplaceAll(pkg, nameSep, sep)}
			units[pkg] = u
		}
		u.add(f)
	}
	for _, src := range orphans {
		pkg := dirPackage(src.Path)
		if units[pkg] == nil {
			pkg = ""
		}
		syntax.Parse(src, syntax.FileSource, r.BagOf(pkg))
	}
	out := slices.SortedFunc(maps.Values(units), func(a, b *Unit) int { return cmp.Compare(a.Name, b.Name) })
	for _, u := range out {
		slices.Sort(u.Imports)
		u.Imports = slices.Compact(u.Imports)
		slices.Sort(u.Layers)
		u.Layers = slices.Compact(u.Layers)
	}
	return out, nil
}

// read reads one file into the set.
func (r *Reader) read(name string) (*source.File, error) {
	abs := path.Join(r.Dir, name)
	data, err := r.FS.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	r.Sums = append(r.Sums, FileSum{Path: name, Sum: sha256.Sum256(data)})
	src, err := r.Set.Add(name, abs, data)
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	return src, nil
}

// parse parses src with a bag of its own and, when it has findings and a package line, again
// into its package's bag; a file without a package line is left to Parse.
func (r *Reader) parse(src *source.File) (*syntax.File, string) {
	own := diag.NewBag(r.Set, "")
	f := syntax.Parse(src, syntax.FileSource, own)
	pkg := qualified(f.Package)
	if pkg != "" && len(own.Findings()) > 0 {
		f = syntax.Parse(src, syntax.FileSource, r.BagOf(pkg))
	}
	return f, pkg
}

// dirPackage is the package named by the directory of a project-relative file (SPEC §3.2).
func dirPackage(name string) string {
	dir := path.Dir(name)
	if dir == currentSeg {
		return ""
	}
	return strings.ReplaceAll(dir, sep, nameSep)
}

func (u *Unit) add(f *syntax.File) {
	u.Files = append(u.Files, f)
	for _, imp := range f.Imports {
		if name := qualified(imp.Path); name != "" {
			u.Imports = append(u.Imports, name)
		}
	}
	if f.FileKind == syntax.FileLayer && f.Layer != nil {
		u.Layers = append(u.Layers, f.Layer.Name)
	}
}
