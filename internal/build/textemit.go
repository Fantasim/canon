package build

import (
	"crypto/sha256"
	"encoding/hex"
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

// listings are the `canon.outputs` of the packages with an `emit text` this build runs, in package order, each naming the text files outputs holds for it: a package whose every optional file is none still has its list (CODEGEN.md §2.9, DECISIONS 326, 336).
func (r *run) listings(outputs []*output, opt BuildOptions, failed bool) []*output {
	names := map[string][]listEntry{}
	for _, o := range outputs {
		if o.Target == ir.TargetText {
			names[o.Package] = append(names[o.Package], listEntry{sum: sumHex(o.Content), path: o.Path})
		}
	}
	var out []*output
	for _, p := range r.ir {
		i := slices.IndexFunc(p.Emits, func(e *ir.Emit) bool { return e.Target == ir.TargetText && !skipped(e, opt.Targets, failed) })
		if i < 0 {
			continue
		}
		rel := path.Join(packageRel(p.Name), outputsName)
		out = append(out, &output{at: r.emitSpan(p, p.Emits[i]), listing: true, Output: Output{
			Path: rel, Abs: project.Join(r.p.dir, rel), Target: ir.TargetText, Package: p.Name, Content: listingOf(p.Dir, names[p.Name]),
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

// listEntry is a line of a `canon.outputs`: the SHA-256 of the file's bytes in lower-case hex, empty in a list written before DECISIONS 336, and its display path.
type listEntry struct {
	sum, path string
}

// sumHex is the SHA-256 of data in lower-case hex.
func sumHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// listingOf is a `canon.outputs`: the marker naming dir, then a line per file, `<sha256 hex>  <display path>`, in byte order of the path, each once (CODEGEN.md §2.9, DECISIONS 336).
func listingOf(dir string, entries []listEntry) []byte {
	entries = slices.Clone(entries)
	slices.SortFunc(entries, func(a, b listEntry) int { return strings.Compare(a.path, b.path) })
	var b strings.Builder
	fmt.Fprintf(&b, textMarkerFormat, dir)
	for _, e := range slices.CompactFunc(entries, func(a, b listEntry) bool { return a.path == b.path }) {
		b.WriteString(e.sum + listSumSep + e.path + lineEnd)
	}
	return []byte(b.String())
}

// ownership is what a build knows of who owns the text files of its packages (CODEGEN.md §2.9).
type ownership struct {
	r      *run
	listed map[string]map[string]string // package -> the display paths its canon.outputs names, each with its recorded SHA-256, read when first needed
	legacy map[string]map[string]bool   // package -> the display paths its own legacy `.canon-text` lists
	gone   []*output                    // the legacy `.canon-text` files the build deletes
}

// owners reads the legacy manifests of the text outputs' directories (CODEGEN.md §2.9 Migration); a manifest that cannot be read is an error.
func (r *run) owners(outputs []*output) (*ownership, error) {
	own := &ownership{r: r, listed: map[string]map[string]string{}, legacy: map[string]map[string]bool{}}
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
	case cannotExist(err):
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
	_, listed := names[o.Path]
	return listed, err
}

// names are the display paths package pkg's marked `canon.outputs` names; none when it has no such file.
func (own *ownership) names(pkg string) (map[string]string, error) {
	if names, ok := own.listed[pkg]; ok {
		return names, nil
	}
	names, err := own.r.listedNames(pkg, isMarkedListing)
	if err == nil {
		own.listed[pkg] = names
	}
	return names, err
}

// isMarkedListing accepts a `canon.outputs` whose first line is the marker of any package: ownership to overwrite (CODEGEN.md §2.9).
func isMarkedListing(content []byte) bool { return firstLineMatches(content, textMarker) }

// namesDir accepts a `canon.outputs` whose first line is the marker naming package pkg's own directory, exactly: the only list whose files a build removes (CODEGEN.md §2.9, DECISIONS 336).
func namesDir(pkg string) func([]byte) bool {
	marker := strings.TrimSuffix(fmt.Sprintf(textMarkerFormat, packageRel(pkg)), lineEnd)
	return func(content []byte) bool { return firstLineIs(content, marker) }
}

// listedNames reads the display paths package pkg's `canon.outputs` names, each with the SHA-256 its line records ("" for a list of the older form), as the previous build wrote it, when accepts takes its content; none when it has no such file.
func (r *run) listedNames(pkg string, accepts func([]byte) bool) (map[string]string, error) {
	rel := path.Join(packageRel(pkg), outputsName)
	data, err := r.p.fs.ReadFile(project.Join(r.p.dir, rel))
	names := map[string]string{}
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, displayError(rel, err)
	case accepts(data):
		for _, e := range listEntries(data) {
			names[e.path] = e.sum
		}
	}
	return names, nil
}

// listEntries are the lines of a `canon.outputs` or legacy `.canon-text` after its marker line: a line `<sha256 hex>  <path>` has its sum, any other line is a bare path; empty lines are left out.
func listEntries(listing []byte) []listEntry {
	lines := strings.Split(string(listing), lineEnd)
	var out []listEntry
	for _, l := range lines[1:] {
		l = strings.TrimSuffix(l, carriageReturn)
		switch m := listLine.FindStringSubmatch(l); {
		case m != nil:
			out = append(out, listEntry{sum: m[1], path: m[listPathGroup]})
		case l != "":
			out = append(out, listEntry{path: l})
		}
	}
	return out
}

// textNames are the paths of listEntries.
func textNames(listing []byte) []string {
	var out []string
	for _, e := range listEntries(listing) {
		out = append(out, e.path)
	}
	return out
}
