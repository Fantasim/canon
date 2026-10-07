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

// textFiles is an `emit text` copy's files: each `@text` file (CODEGEN.md §2.9).
func (r *run) textFiles(p *ir.Package, _ *ir.Emit) ([]ir.File, error) {
	i := slices.IndexFunc(r.cps, func(cp *check.Package) bool { return cp.Path == p.Name })
	if i < 0 {
		return nil, internal(fmt.Errorf(fmtUnchecked, p.Name))
	}
	files, err := ir.TextFiles(r.cps[i], p)
	if err != nil {
		return nil, internal(err)
	}
	return files, nil
}

// packageRel is the directory of the package named name, relative to the project root: the one holding its canon.lock (LOCK.md §2.1).
func packageRel(name string) string {
	return strings.ReplaceAll(name, qnameSep, pathSep)
}

// packageDir is the directory of the package named name in the run's IR, as its markers name it (CODEGEN.md §2.4); false when the run has no such package. A caller holding an output of the run always finds its package, since outputs come from r.ir.
func (r *run) packageDir(name string) (string, bool) {
	i := slices.IndexFunc(r.ir, func(p *ir.Package) bool { return p.Name == name })
	if i < 0 {
		return "", false
	}
	return r.ir[i].Dir, true
}

// listings are the `canon.outputs` of the packages whose text files outputs holds, in package order (CODEGEN.md §2.9, DECISIONS 326).
func (r *run) listings(outputs []*output) []*output {
	var order []string
	names := map[string][]string{}
	first := map[string]*output{}
	for _, o := range outputs {
		if o.Target != ir.TargetText {
			continue
		}
		if _, seen := first[o.Package]; !seen {
			order, first[o.Package] = append(order, o.Package), o
		}
		names[o.Package] = append(names[o.Package], o.Path)
	}
	out := make([]*output, 0, len(order))
	for _, pkg := range order {
		dir, _ := r.packageDir(pkg) // found: pkg is the package of an output of r.ir
		rel := path.Join(packageRel(pkg), outputsName)
		out = append(out, &output{at: first[pkg].at, listing: true, Output: Output{
			Path: rel, Abs: project.Join(r.p.dir, rel), Target: ir.TargetText, Package: pkg, Content: listingOf(dir, names[pkg]),
		}})
	}
	return out
}

// staleListings are the `canon.outputs` of the selected packages with no `emit text` that still carry the marker, or a set-aside name of one an earlier build left: removed, so that a stale list never grants ownership later (CODEGEN.md §2.9).
func (r *run) staleListings() ([]*output, error) {
	var out []*output
	for _, cp := range r.cps {
		if r.hasTextEmit(cp.Path) {
			continue
		}
		o, err := r.staleListing(cp.Path)
		if err != nil {
			return nil, err
		}
		if o != nil {
			out = append(out, o)
		}
	}
	return out, nil
}

// staleListing is the removal of package pkg's marked `canon.outputs`, or of its set-aside name; nil when there is none.
func (r *run) staleListing(pkg string) (*output, error) {
	rel := path.Join(packageRel(pkg), outputsName)
	abs := project.Join(r.p.dir, rel)
	isMarked := func(content []byte) bool { return firstLineMatches(content, textMarker) }
	data, err := r.p.fs.ReadFile(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		gone, err := r.leftover(abs, rel, isMarked)
		if gone != nil {
			gone.Package = pkg
		}
		return gone, err
	case err != nil:
		return nil, displayError(rel, err)
	case isMarked(data):
		return &output{remove: true, Output: Output{Path: rel, Abs: abs, Target: ir.TargetText, Package: pkg, Status: StatusWritten}}, nil
	}
	return nil, nil
}

// hasTextEmit reports a package that declares an `emit text`; the IR may omit a package with no emit, so the checked packages are walked instead.
func (r *run) hasTextEmit(name string) bool {
	return slices.ContainsFunc(r.ir, func(p *ir.Package) bool {
		return p.Name == name && slices.ContainsFunc(p.Emits, func(e *ir.Emit) bool { return e.Target == ir.TargetText })
	})
}

// listingOf is a `canon.outputs`: the marker naming dir, then the file names in byte order, each once (CODEGEN.md §2.9).
func listingOf(dir string, names []string) []byte {
	names = slices.Clone(names)
	slices.Sort(names)
	var b strings.Builder
	fmt.Fprintf(&b, textMarkerFormat, dir)
	for _, n := range slices.Compact(names) {
		b.WriteString(n + lineEnd)
	}
	return []byte(b.String())
}

// ownership is what a build knows of who owns the text files of its packages (CODEGEN.md §2.9).
type ownership struct {
	r      *run
	listed map[string]map[string]bool // package -> the display paths its canon.outputs names, read when first needed
	legacy map[string]map[string]bool // package -> the display paths its own legacy `.canon-text` lists
	gone   []*output                  // the legacy `.canon-text` files the build deletes
}

// owners reads the legacy manifests of the text outputs' directories (CODEGEN.md §2.9 Migration); a manifest that cannot be read is an error.
func (r *run) owners(outputs []*output) (*ownership, error) {
	own := &ownership{r: r, listed: map[string]map[string]bool{}, legacy: map[string]map[string]bool{}}
	seen := map[string]bool{}
	for _, o := range outputs {
		dir := project.DirOf(o.Abs)
		if o.Target != ir.TargetText || o.listing || o.remove || seen[o.Package+pathSep+dir] {
			continue
		}
		seen[o.Package+pathSep+dir] = true
		if err := own.readLegacy(o, outputs); err != nil {
			return nil, err
		}
	}
	return own, nil
}

// readLegacy records the `.canon-text` beside o as ownership and as a file to delete when its marker names o's package directory and no output is written at its path.
func (own *ownership) readLegacy(o *output, outputs []*output) error {
	pkgDir, ok := own.r.packageDir(o.Package)
	abs := project.Join(project.DirOf(o.Abs), legacyListing)
	if !ok || slices.ContainsFunc(outputs, func(w *output) bool { return strings.EqualFold(w.Abs, abs) }) {
		return nil
	}
	display := path.Join(path.Dir(o.Path), legacyListing)
	data, err := own.r.p.fs.ReadFile(abs)
	marker := strings.TrimSuffix(fmt.Sprintf(textMarkerFormat, pkgDir), lineEnd)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return own.sweep(o, abs, display, func(content []byte) bool { return firstLineIs(content, marker) })
	case err != nil:
		return displayError(display, err)
	case !firstLineIs(data, marker):
		return nil
	}
	if own.legacy[o.Package] == nil {
		own.legacy[o.Package] = map[string]bool{}
	}
	for _, name := range textNames(data) {
		own.legacy[o.Package][path.Join(path.Dir(o.Path), name)] = true
	}
	own.gone = append(own.gone, &output{at: o.at, remove: true, Output: Output{
		Path: display, Abs: abs, Target: ir.TargetText, Package: o.Package, Status: StatusWritten,
	}})
	return nil
}

// sweep deletes what an earlier build left of the file abs: the name it was set aside under, when it holds a file the marked test accepts (CODEGEN.md §2.9).
func (own *ownership) sweep(o *output, abs, display string, marked func([]byte) bool) error {
	gone, err := own.r.leftover(abs, display, marked)
	if gone != nil {
		gone.Package, gone.at = o.Package, o.at
		own.gone = append(own.gone, gone)
	}
	return err
}

// leftover is the deletion of the set-aside name of the file abs (display), when one is there and its content passes marked; nil when there is none.
func (r *run) leftover(abs, display string, marked func([]byte) bool) (*output, error) {
	aside := tempOf(abs)
	data, err := r.p.fs.ReadFile(aside)
	shown := path.Join(path.Dir(display), path.Base(aside))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, displayError(shown, err)
	case !marked(data):
		return nil, nil
	}
	return &output{remove: true, Output: Output{Path: shown, Abs: aside, Target: ir.TargetText, Status: StatusWritten}}, nil
}

// owned reports an output canon may overwrite: marked, or a text file its package's marked `canon.outputs` names, or a legacy manifest of it lists (CODEGEN.md §2.4, §2.9); a manifest that cannot be read is an error.
func (own *ownership) owned(o *output, old []byte) (bool, error) {
	switch {
	case o.Target != ir.TargetText:
		return marked(o.Output, old), nil
	case o.listing:
		return firstLineMatches(old, textMarker), nil
	case own.legacy[o.Package][o.Path]:
		return true, nil
	}
	names, err := own.names(o.Package)
	return names[o.Path], err
}

// names are the display paths package pkg's marked `canon.outputs` names; none when it has no such file.
func (own *ownership) names(pkg string) (map[string]bool, error) {
	if names, ok := own.listed[pkg]; ok {
		return names, nil
	}
	rel := path.Join(packageRel(pkg), outputsName)
	data, err := own.r.p.fs.ReadFile(project.Join(own.r.p.dir, rel))
	names := map[string]bool{}
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, displayError(rel, err)
	case firstLineMatches(data, textMarker):
		for _, n := range textNames(data) {
			names[n] = true
		}
	}
	own.listed[pkg] = names
	return names, nil
}

// textNames are the lines of a `canon.outputs` or legacy `.canon-text` after its marker line.
func textNames(listing []byte) []string {
	lines := strings.Split(string(listing), lineEnd)
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, carriageReturn)
	}
	return lines[1:]
}
