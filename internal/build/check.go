package build

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Result is one check: the selected packages and their findings, the project's own too (API.md R2).
type Result struct {
	Packages []string
	Findings
	Revision string
}

// Units is the scan of a snapshot: every package of the project, in name order (API.md §5.5).
type Units struct {
	Units    []*project.Unit
	Revision string
}

// snapshot is what one call reads: every file under a new file set, and a bag per package.
type snapshot struct {
	p      *Project
	set    *source.FileSet
	own    *diag.Bag
	bags   map[string]*diag.Bag
	proj   *project.Project
	layout *project.Layout
	names  []string
	units  []*project.Unit
	sums   []project.FileSum
}

// Packages scans and parses the project and lists its packages (API.md §5.5, O4).
func (p *Project) Packages(ctx context.Context) (*Units, error) {
	s, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.checkLayers(s.units); err != nil {
		return nil, err
	}
	return &Units{Units: s.units, Revision: s.revision()}, nil
}

// Revision is the revision of what is on disk now (API.md S1, S3), whether project.canon
// checks or not; a file, or the listing of the file set, that cannot be read is marked so.
func (p *Project) Revision(ctx context.Context) (string, error) {
	lines := []digest{p.digestOf(project.FileName)}
	names, err := project.Scan(p.fs, p.dir)
	if err != nil {
		lines = append(lines, digest{path: listingMark, text: unreadMark})
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		lines = append(lines, p.digestOf(name))
	}
	return revisionOf(lines), nil
}

// digest is one line of a read-set listing: a file's SHA-256 in hex, or unreadMark.
type digest struct {
	path, text string
}

func (p *Project) digestOf(name string) digest {
	data, err := p.fs.ReadFile(path.Join(p.dir, name))
	if err != nil {
		return digest{path: name, text: unreadMark}
	}
	sum := sha256.Sum256(data)
	return digest{path: name, text: hex.EncodeToString(sum[:])}
}

// Check parses the selected packages and their imports, then checks them with the Checker; an
// unknown selector or layer is a *project.UnknownError (API.md R1, O4).
func (p *Project) Check(ctx context.Context, selectors []string) (*Result, error) {
	s, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	selected, err := project.Select(s.units, selectors)
	if err != nil {
		return nil, err
	}
	loaded := imported(s.units, selected)
	if err := p.checkLayers(loaded); err != nil {
		return nil, err
	}
	if p.opt.Checker != nil {
		p.opt.Checker(ctx, s.proj, filesOf(loaded), s.bagsOf(loaded))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res := &Result{Revision: s.revision()}
	var bags []*diag.Bag
	for _, u := range selected {
		res.Packages = append(res.Packages, u.Name)
		bags = append(bags, s.bag(u.Name))
	}
	res.Findings = collect(s.set, s.own, bags...)
	return res, nil
}

// load reads a new snapshot: project.canon, then every source file, parsed.
func (p *Project) load(ctx context.Context) (*snapshot, error) {
	s, err := p.open()
	if err != nil {
		return nil, err
	}
	r := &project.Reader{FS: p.fs, Dir: p.dir, Set: s.set, BagOf: s.bag}
	if s.units, err = r.Parse(ctx, s.names); err != nil {
		return nil, err
	}
	s.sums = append(s.sums, r.Sums...)
	project.CheckStudio(s.proj, s.units, s.own)
	return s, nil
}

// bag is the bag of package pkg; "" is the project's own.
func (s *snapshot) bag(pkg string) *diag.Bag {
	if pkg == "" {
		return s.own
	}
	if s.bags[pkg] == nil {
		s.bags[pkg] = s.p.newBag(s.set, pkg)
	}
	return s.bags[pkg]
}

func (s *snapshot) bagsOf(units []*project.Unit) map[string]*diag.Bag {
	out := map[string]*diag.Bag{}
	for _, u := range units {
		out[u.Name] = s.bag(u.Name)
	}
	return out
}

func (s *snapshot) revision() string {
	lines := make([]digest, len(s.sums))
	for i, f := range s.sums {
		lines[i] = digest{path: f.Path, text: hex.EncodeToString(f.Sum[:])}
	}
	return revisionOf(lines)
}

// revisionOf is "r1:" and the SHA-256 of the listing of the files read (API.md S3).
func revisionOf(lines []digest) string {
	sorted := slices.SortedFunc(slices.Values(lines), func(a, b digest) int { return cmp.Compare(a.path, b.path) })
	var listing strings.Builder
	for _, l := range sorted {
		listing.WriteString(l.path + listingSep + l.text + listingEnd)
	}
	sum := sha256.Sum256([]byte(listing.String()))
	return revisionPrefix + hex.EncodeToString(sum[:])
}

// imported is selected and every package they import, directly or not, in name order; an
// import of no package is check's E2003.
func imported(all, selected []*project.Unit) []*project.Unit {
	out := slices.Clone(selected)
	for i := 0; i < len(out); i++ {
		for _, name := range out[i].Imports {
			j := slices.IndexFunc(all, func(u *project.Unit) bool { return u.Name == name })
			if j >= 0 && !slices.Contains(out, all[j]) {
				out = append(out, all[j])
			}
		}
	}
	slices.SortFunc(out, func(a, b *project.Unit) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

func filesOf(units []*project.Unit) []*syntax.File {
	var out []*syntax.File
	for _, u := range units {
		out = append(out, u.Files...)
	}
	return out
}

// checkLayers refuses a layer that no loaded package has a file for (API.md O4, LAY-01).
func (p *Project) checkLayers(loaded []*project.Unit) error {
	for _, name := range p.opt.Layers {
		if !slices.ContainsFunc(loaded, func(u *project.Unit) bool { return slices.Contains(u.Layers, name) }) {
			return &project.UnknownError{Err: ErrUnknownLayer, Name: name}
		}
	}
	return nil
}
