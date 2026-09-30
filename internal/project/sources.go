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

type sha256Sum = [sha256.Size]byte

// FileSum is the SHA-256 of a file as read, by display path (API.md S3).
type FileSum struct {
	Path string
	Sum  sha256Sum
}

// Reader parses the source files of the project in Dir into Set, each file's findings going to
// the bag BagOf gives for its package. A file without a package line joins no package: its
// findings go to its directory's package when one exists, else to the project's own, "".
type Reader struct {
	FS    FS
	Dir   string
	Set   *source.FileSet
	BagOf func(pkg string) *diag.Bag
	Reuse *Reuse    // nil: parse every file; else unchanged files return their tree (NFR-02); its set is Set
	Sums  []FileSum // every file read, in reading order
}

// Parse reads and parses the files names (Scan's) and groups them by package, in name order.
func (r *Reader) Parse(ctx context.Context, names []string) ([]*Unit, error) {
	if r.Reuse != nil && r.Reuse.set != r.Set {
		return nil, ErrReuseSet
	}
	units := map[string]*Unit{}
	var orphans []parsedFile
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p, err := r.file(name)
		if err != nil {
			return nil, err
		}
		if p.pkg == "" {
			orphans = append(orphans, p)
			continue
		}
		u := units[p.pkg]
		if u == nil {
			u = &Unit{Name: p.pkg, Dir: strings.ReplaceAll(p.pkg, nameSep, sep)}
			units[p.pkg] = u
		}
		u.add(p.file)
	}
	for _, p := range orphans {
		pkg := dirPackage(p.src.Path)
		if units[pkg] == nil {
			pkg = ""
		}
		r.report(p, pkg)
	}
	r.Reuse.retain(names)
	out := slices.SortedFunc(maps.Values(units), func(a, b *Unit) int { return cmp.Compare(a.Name, b.Name) })
	for _, u := range out {
		slices.Sort(u.Imports)
		u.Imports = slices.Compact(u.Imports)
		slices.Sort(u.Layers)
		u.Layers = slices.Compact(u.Layers)
	}
	return out, nil
}

// file reads one file and parses it into the set, unless Reuse holds the parse of that content.
func (r *Reader) file(name string) (parsedFile, error) {
	abs := Join(r.Dir, name)
	data, err := r.FS.ReadFile(abs)
	if err != nil {
		return parsedFile{}, fmt.Errorf(fmtWrap, err)
	}
	sum := sha256.Sum256(data)
	r.Sums = append(r.Sums, FileSum{Path: name, Sum: sum})
	if p, ok := r.Reuse.lookup(name, sum); ok {
		r.replay(p)
		return p, nil
	}
	src, err := r.Set.Add(name, abs, data)
	if err != nil {
		return parsedFile{}, fmt.Errorf(fmtWrap, err)
	}
	p := r.parse(src)
	r.Reuse.store(name, sum, p)
	return p, nil
}

// parse parses src with a bag of its own and, when it has findings and a package line, again
// into its package's bag; a file without a package line is left to Parse.
func (r *Reader) parse(src *source.File) parsedFile {
	own := diag.NewBag(r.Set, "")
	f := syntax.Parse(src, syntax.FileSource, own)
	p := parsedFile{src: src, file: f, pkg: qualified(f.Package), findings: len(own.Findings()) > 0}
	if p.pkg != "" && p.findings {
		p.file = syntax.Parse(src, syntax.FileSource, r.BagOf(p.pkg))
	}
	return p
}

// replay gives the package's bag the findings of a file taken from Reuse: parsing it again
// reports them as a fresh parse does, at the same spans, and its tree is dropped for the
// stored one. A file without findings, or without a package line, has nothing to replay here.
func (r *Reader) replay(p parsedFile) {
	if p.pkg != "" && p.findings {
		syntax.Parse(p.src, syntax.FileSource, r.BagOf(p.pkg))
	}
}

// report parses a file without a package line into the bag of pkg, which takes its findings; a
// file taken from Reuse is parsed again only when it has findings.
func (r *Reader) report(p parsedFile, pkg string) {
	bag := r.BagOf(pkg)
	if !p.reused || p.findings {
		syntax.Parse(p.src, syntax.FileSource, bag)
	}
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
