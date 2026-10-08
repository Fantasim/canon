package build

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"syscall"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/wire"
)

// staleJudge decides which listed files a build removes: the ones the package's previous `canon.outputs` lists and this build no longer writes, under the guards of CODEGEN.md §2.9 (DECISIONS 336).
type staleJudge struct {
	r         *run
	opt       BuildOptions
	written   map[string]string          // lower-cased path to exact path: every output of this build, then the removals made so far
	protected map[string]bool            // lower-cased: an input of the run, or a canon.lock or canon.outputs
	claimed   map[string]bool            // lower-cased paths of the emits this build does not run, made on first use
	elsewhere map[string]map[string]bool // package -> lower-cased paths of its own marked canon.outputs, read when first needed
}

// staleFiles are the removals of the files the previous `canon.outputs` of each selected package lists (when its marker names the package's own directory) and this build does not write.
func (r *run) staleFiles(opt BuildOptions, outputs []*output) ([]*output, error) {
	j := r.newStaleJudge(opt, outputs)
	var out []*output
	for _, cp := range r.cps {
		names, err := r.listedNames(cp.Path, namesDir(cp.Path))
		if err != nil {
			return nil, err
		}
		for _, name := range slices.Sorted(maps.Keys(names)) {
			gone, err := j.stale(cp.Path, name, names[name])
			if err != nil {
				return nil, err
			}
			if gone != nil {
				out = append(out, gone)
			}
		}
	}
	return out, nil
}

// newStaleJudge holds what the run writes and reads: its outputs, and as inputs project.canon, project.local.canon, every source, lock and list, and every file a load read.
func (r *run) newStaleJudge(opt BuildOptions, outputs []*output) *staleJudge {
	j := &staleJudge{r: r, opt: opt, written: map[string]string{}, protected: map[string]bool{}, elsewhere: map[string]map[string]bool{}}
	for _, o := range outputs {
		j.written[strings.ToLower(o.Abs)] = o.Abs
	}
	for key := range r.reserved() { //canon:unordered a set
		j.protected[key] = true
	}
	for _, f := range r.s.sums {
		j.protected[strings.ToLower(project.Join(r.p.dir, f.Path))] = true
	}
	j.protected[strings.ToLower(project.Join(r.p.dir, project.LocalFileName))] = true
	for abs := range r.inputs().loaded { //canon:unordered a set
		j.protected[strings.ToLower(abs)] = true
	}
	return j
}

// stale is the removal of the file the display path name of package pkg's previous list names with the SHA-256 sum, or of the set-aside name an earlier removal left; nil when the build keeps the path or leaves it alone. A file is removed only if its bytes are those the list recorded: a list of the older form, with no sum, removes nothing (DECISIONS 336).
func (j *staleJudge) stale(pkg, name, sum string) (*output, error) {
	r := j.r
	at, ok := r.s.layout.Resolve(name, "", source.Span{}, diag.NewBag(nil, pkg))
	if !ok || at.Dir || j.protected[strings.ToLower(at.Abs)] || !check.ValidTextName(path.Base(at.Abs)) || j.writtenHere(at.Abs) {
		return nil, nil
	}
	info, err := r.p.fs.Stat(at.Abs)
	switch {
	case cannotExist(err):
		return r.asideOf(pkg, at, sum)
	case err != nil:
		return nil, displayError(name, err)
	case !info.Mode().IsRegular(): // a directory, a device, a pipe: not a file canon wrote
		return nil, nil
	}
	if sum == "" {
		return nil, nil
	}
	if keep, err := j.kept(pkg, name, at.Abs, sum); keep || err != nil {
		return nil, err
	}
	j.written[strings.ToLower(at.Abs)] = at.Abs
	return &output{remove: true, Output: Output{Path: name, Abs: at.Abs, Target: ir.TargetText, Package: pkg, Status: StatusWritten}}, nil
}

// asideOf is the removal of the set-aside name of at an earlier removal left, when the line recorded a sum and the aside's bytes still hash to it; nil when there is none, or the line has no sum.
func (r *run) asideOf(pkg string, at project.Path, sum string) (*output, error) {
	aside := tempOf(at.Abs)
	shown := path.Join(path.Dir(at.Display), path.Base(aside))
	info, err := r.p.fs.Stat(aside)
	switch {
	case sum == "" || cannotExist(err):
		return nil, nil
	case err != nil:
		return nil, displayError(shown, err)
	case !info.Mode().IsRegular():
		return nil, nil
	}
	got, err := r.sumOf(aside)
	switch {
	case cannotExist(err) || got != sum && err == nil:
		return nil, nil
	case err != nil:
		return nil, displayError(shown, err)
	}
	return &output{remove: true, Output: Output{Path: shown, Abs: aside, Target: ir.TargetText, Package: pkg, Status: StatusWritten}}, nil
}

// writtenHere reports the path abs one of this build's outputs, or one that differs from an output in case only and may be the same file: it is another file only where the output's name resolves to nothing yet (a case-sensitive file system, so a rename leaves the old name), or where the directory listing holds both exact names (CODEGEN.md §2.9, DECISIONS 336).
func (j *staleJudge) writtenHere(abs string) bool {
	exact, ok := j.written[strings.ToLower(abs)]
	if !ok || exact == abs {
		return ok
	}
	if _, err := j.r.p.fs.Stat(abs); err != nil {
		return true
	}
	if _, err := j.r.p.fs.Stat(exact); cannotExist(err) {
		return false
	}
	return !j.bothListed(abs, exact)
}

// bothListed reports the two paths, equal but for case, both named by the listing of the directory holding the first segment in which they differ: two files, not one file under two spellings.
func (j *staleJudge) bothListed(a, b string) bool {
	x, y := strings.Split(a, pathSep), strings.Split(b, pathSep)
	i := 0
	for i < len(x) && i < len(y) && x[i] == y[i] {
		i++
	}
	if len(x) != len(y) || i == len(x) {
		return false
	}
	entries, err := j.r.p.fs.ReadDir(cmp.Or(strings.Join(x[:i], pathSep), pathSep))
	if err != nil {
		return false
	}
	listed := map[string]bool{}
	for _, e := range entries {
		listed[e.Name()] = true
	}
	return listed[x[i]] && listed[y[i]]
}

// kept reports a listed file that exists and is not the build's to remove: it holds a generated file's marker, another package lists it, an emit this build does not run writes it, or its bytes are not those the list recorded, which only the last check reads in full (CODEGEN.md §2.9, DECISIONS 336).
func (j *staleJudge) kept(pkg, name, abs, sum string) (bool, error) {
	head, err := j.r.readHead(abs)
	switch {
	case cannotExist(err):
		return true, nil
	case err != nil:
		return false, displayError(name, err)
	case generated(abs, head):
		return true, nil
	}
	other, err := j.listedElsewhere(pkg, abs)
	if other || err != nil || j.claimedSet()[strings.ToLower(abs)] {
		return true, err
	}
	got, err := j.r.sumOf(abs)
	switch {
	case cannotExist(err):
		return true, nil
	case err != nil:
		return false, displayError(name, err)
	}
	return got != sum, nil
}

// streamFS is a file system that opens a file as a stream.
type streamFS interface {
	OpenRead(name string) (io.ReadCloser, error)
}

// readHead is the first sniffBytes bytes of the file abs.
func (r *run) readHead(abs string) ([]byte, error) {
	fsys, ok := r.p.fs.(streamFS)
	if !ok {
		data, err := r.p.fs.ReadFile(abs)
		return data[:min(len(data), sniffBytes)], err
	}
	f, err := fsys.OpenRead(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head, err := io.ReadAll(io.LimitReader(f, sniffBytes))
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	return head, nil
}

// sumOf is the SHA-256 of the file abs in lower-case hex, the file streamed where the file system can, never held whole.
func (r *run) sumOf(abs string) (string, error) {
	fsys, ok := r.p.fs.(streamFS)
	if !ok {
		data, err := r.p.fs.ReadFile(abs)
		return sumHex(data), err
	}
	f, err := fsys.OpenRead(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf(fmtWrap, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// generated reports the start of a file that carries a canon marker: the first-line marker of any target (CODEGEN.md §2.4: Go, C++, TS, a runtime file, a text list), or for a JSON file a `$schema` key. A text file carries none.
func generated(abs string, data []byte) bool {
	head := data[:min(len(data), sniffBytes)]
	return firstLineMatches(head, codeMarker) || firstLineMatches(head, textMarker) || path.Ext(abs) == ir.JSONExt && bytes.Contains(head, []byte(`"`+wire.KeySchema+`"`))
}

// cannotExist reports an error that says the path holds nothing: absent, or a parent that is not a directory.
func cannotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// listedElsewhere reports abs listed by the marked `canon.outputs` of a package other than pkg, selected or not.
func (j *staleJudge) listedElsewhere(pkg, abs string) (bool, error) {
	for _, u := range j.r.s.units {
		if u.Name == pkg {
			continue
		}
		paths, err := j.listOf(u.Name)
		if err != nil {
			return false, err
		}
		if paths[strings.ToLower(abs)] {
			return true, nil
		}
	}
	return false, nil
}

// listOf is the lower-cased paths package pkg's marked `canon.outputs` names, read once.
func (j *staleJudge) listOf(pkg string) (map[string]bool, error) {
	if paths, ok := j.elsewhere[pkg]; ok {
		return paths, nil
	}
	names, err := j.r.listedNames(pkg, isMarkedListing)
	if err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	for name := range names { //canon:unordered a set
		if at, ok := j.r.s.layout.Resolve(name, "", source.Span{}, diag.NewBag(nil, pkg)); ok {
			paths[strings.ToLower(at.Abs)] = true
		}
	}
	j.elsewhere[pkg] = paths
	return paths, nil
}

// claimedSet is the lower-cased path of every file the emits of this build's packages write that the build does not run (a target `--target` left out): generated only when a listed file needs the answer, a failure leaving its paths out.
func (j *staleJudge) claimedSet() map[string]bool {
	if j.claimed != nil {
		return j.claimed
	}
	j.claimed = map[string]bool{}
	for _, p := range j.r.ir {
		for _, e := range p.Emits {
			if len(j.opt.Targets) == 0 || slices.Contains(j.opt.Targets, e.Target) || e.Dir == "" {
				continue
			}
			for _, o := range j.r.emitPaths(p, e) {
				j.claimed[strings.ToLower(o.Abs)] = true
			}
		}
	}
	return j.claimed
}

// emitPaths are the outputs of e without running its view: the files its generator makes, or the one file of a view.
func (r *run) emitPaths(p *ir.Package, e *ir.Emit) []*output {
	cp := ir.CopyOf(r.s.proj, p, e)
	files := []ir.File{{Path: e.FileName}}
	if e.Target != ir.TargetView {
		var err error
		if err = complete(cp); err == nil {
			files, err = r.generate(cp, e)
		}
		if err != nil {
			return nil
		}
	}
	out, err := r.outputs(cp, e, files)
	if err != nil {
		return nil
	}
	return out
}
