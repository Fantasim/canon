package load

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io/fs"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Inputs is what one load read and reported itself, for the memo to replay it (IMPLEMENTATION-PLAN §7.6).
type Inputs struct {
	places []string
	calls  []fsCall
	found  []*diag.Builder
	globs  []globMatch
}

// globMatch is a load.dir pattern's display and its matches' displays, in match order.
type globMatch struct {
	pattern string
	matched []string
}

// fsCall is one file-system call of a load: its kind, its name, for a source the display it
// entered the file set under, and its answer.
type fsCall struct {
	kind    callKind
	name    string
	display string
	answer  answer
}

// answer is a call's answer as far as a load tells answers apart: its error's cause, a stat's
// type bits, the SHA-256 of a file's content, a listing or a link's target.
type answer struct {
	failed bool
	cause  diag.Kind
	mode   fs.FileMode
	sum    [sha256.Size]byte
}

// callsAgain makes one recorded call again through the Loader's FS: true when it answers alike.
var callsAgain = [...]func(*Loader, fsCall) bool{
	callStat:   (*Loader).statAgain,
	callList:   (*Loader).listAgain,
	callLink:   (*Loader).linkAgain,
	callSource: (*Loader).sourceAgain,
}

// Loaded is a recorded load's value, whether it holds, and the inputs a replay needs: none when no
// replay could reproduce it (a failure, a scratch request's, load.defines, whose header all loads share).
type Loaded struct {
	Value  value.Value
	OK     bool
	Inputs *Inputs
}

// Recorded is Load, recorded. A load that holds reports only its own warnings, which are kept: any
// other finding, jsonsrc's and the decoder's included, fails it.
func (l *Loader) Recorded(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (Loaded, error) {
	if req.Scratch || e.Method != nil && e.Method.Name == formDefines {
		v, ok, err := l.Load(ctx, req, e, t)
		return Loaded{Value: v, OK: ok}, err
	}
	outer := l.rec
	in := &Inputs{places: l.places()}
	l.rec, req.found = in, &in.found
	v, ok, err := l.load(ctx, req, e, t)
	l.rec = outer
	out := Loaded{Value: v, OK: ok}
	if err == nil && ok {
		out.Inputs = in
	}
	return out, err
}

// Replay makes in's calls again through l.FS, in order, as the load made them: false at the first
// whose answer differs. Each source read again enters the set as the load's did (Loader.Add).
func (l *Loader) Replay(in *Inputs) bool {
	outer := l.rec
	l.rec = nil
	defer func() { l.rec = outer }()
	if !slices.Equal(in.places, l.places()) {
		return false
	}
	for _, c := range in.calls {
		if !callsAgain[c.kind](l, c) {
			return false
		}
	}
	for _, g := range in.globs {
		l.tellGlob(g)
	}
	return true
}

// globbed tells Globbed a load.dir's matches, and keeps them when the load is recorded.
func (l *Loader) globbed(pattern string, matches []matchFile) {
	if l.Globbed == nil && l.rec == nil {
		return
	}
	g := globMatch{pattern: pattern, matched: make([]string, len(matches))}
	for i, m := range matches {
		g.matched[i] = m.Display
	}
	if l.rec != nil {
		l.rec.globs = append(l.rec.globs, g)
	}
	l.tellGlob(g)
}

// tellGlob is Globbed of g, when set.
func (l *Loader) tellGlob(g globMatch) {
	if l.Globbed != nil {
		l.Globbed(g.pattern, slices.Clone(g.matched))
	}
}

// Report reports into bag the findings the load reported itself.
func (in *Inputs) Report(bag *diag.Bag) {
	for _, b := range in.found {
		b.Report(bag)
	}
}

// places is the project directory, then each root's. A replay compares directories only: the
// roots' names are project.canon's, whose hash is in the check key, and Options.Roots is fixed
// per cache (log-2026-09-29 M4 P3-r).
func (l *Loader) places() []string {
	if l.Layout == nil {
		return nil
	}
	return append([]string{l.Layout.Dir}, l.Layout.RootDirs()...)
}

// report reports b into req's bag, and keeps it when the load is recorded.
func (req Request) report(b *diag.Builder) {
	b.Report(req.Bag)
	if req.found != nil {
		*req.found = append(*req.found, b.Detached())
	}
}

// note keeps a call the recorded load made.
func (l *Loader) note(kind callKind, name, display string, a func() answer) {
	if l.rec != nil {
		l.rec.calls = append(l.rec.calls, fsCall{kind: kind, name: name, display: display, answer: a()})
	}
}

// stat is l.FS.Stat, recorded.
func (l *Loader) stat(name string) (fs.FileInfo, error) {
	info, err := l.FS.Stat(name)
	l.note(callStat, name, "", func() answer { return statAnswer(info, err) })
	return info, err
}

// readDir is dir's entries, empty (not an error) when dir does not exist (WIRE.md §6.5); recorded.
func (l *Loader) readDir(dir string) ([]fs.DirEntry, error) {
	entries, err := l.FS.ReadDir(dir)
	l.note(callList, dir, "", func() answer { return listAnswer(entries, err) })
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return entries, err
}

// evalSymlinks is name's real path through l.FS (project.EvalSymlinks), recorded.
func (l *Loader) evalSymlinks(name string) (string, error) {
	real, err := project.EvalSymlinks(l.FS, name)
	l.note(callLink, name, "", func() answer { return textAnswer(real, err) })
	return real, err
}

// readSource reads abs into the set as display, recorded; E7004 when it cannot (WIRE.md §6.1).
func (l *Loader) readSource(display, abs string, req Request) (*source.File, []byte, bool) {
	data, err := l.FS.ReadFile(abs)
	var sum [sha256.Size]byte
	if err == nil && (l.rec != nil || l.Add != nil) { // hashed once, for the record and the cache
		sum = sha256.Sum256(data)
	}
	l.note(callSource, abs, display, func() answer { return sumAnswer(sum, err) })
	var src *source.File
	if err == nil {
		src, err = l.add(display, abs, data, sum)
	}
	if err != nil {
		req.report(diag.E7004.At(req.Span, display, causeOf(err)))
		return nil, nil, false
	}
	return src, data, true
}

// add is the file set's file of data, whose SHA-256 is sum when Add is set, through Add.
func (l *Loader) add(display, abs string, data []byte, sum [sha256.Size]byte) (*source.File, error) {
	if l.Add != nil {
		return l.Add(display, abs, data, sum)
	}
	return l.Set.Add(display, abs, data)
}

func (l *Loader) statAgain(c fsCall) bool {
	info, err := l.FS.Stat(c.name)
	return statAnswer(info, err) == c.answer
}

func (l *Loader) listAgain(c fsCall) bool {
	entries, err := l.FS.ReadDir(c.name)
	return listAnswer(entries, err) == c.answer
}

func (l *Loader) linkAgain(c fsCall) bool {
	real, err := project.EvalSymlinks(l.FS, c.name)
	return textAnswer(real, err) == c.answer
}

// sourceAgain takes the file again, unread when the FS knows its SHA-256 and Kept keeps that
// content (log-2026-09-29 P18), else read; the same content enters the set as the load's did.
func (l *Loader) sourceAgain(c fsCall) bool {
	if sum, known := l.fixedSum(c.name); known {
		if sumAnswer(sum, nil) != c.answer {
			return false
		}
		if _, ok := l.Kept(c.display, c.name, sum); ok {
			return true
		}
	}
	data, err := l.FS.ReadFile(c.name)
	if err != nil {
		return failure(err) == c.answer
	}
	sum := sha256.Sum256(data)
	if sumAnswer(sum, nil) != c.answer {
		return false
	}
	_, err = l.add(c.display, c.name, data, sum)
	return err == nil
}

// failure is the answer of a call that failed: its cause, as the load tells causes apart.
func failure(err error) answer {
	return answer{failed: true, cause: causeOf(err)}
}

func statAnswer(info fs.FileInfo, err error) answer {
	if err != nil {
		return failure(err)
	}
	return answer{mode: info.Mode().Type()}
}

// listAnswer sums each entry's name and type bits, in the listing's order.
func listAnswer(entries []fs.DirEntry, err error) answer {
	if err != nil {
		return failure(err)
	}
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.Name()))
		h.Write(binary.BigEndian.AppendUint32([]byte{0}, uint32(e.Type())))
	}
	a := answer{}
	copy(a.sum[:], h.Sum(nil))
	return a
}

func textAnswer(text string, err error) answer {
	return contentAnswer([]byte(text), err)
}

func contentAnswer(data []byte, err error) answer {
	if err != nil {
		return failure(err)
	}
	return answer{sum: sha256.Sum256(data)}
}

// sumAnswer is contentAnswer of a content whose SHA-256 is sum.
func sumAnswer(sum [sha256.Size]byte, err error) answer {
	if err != nil {
		return failure(err)
	}
	return answer{sum: sum}
}
