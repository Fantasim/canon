package edit

import (
	"bytes"
	"cmp"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// work is what one operation changes: edits per file, files created, deleted and renamed, and
// what it gives back (its inverses, the packages it writes, what its cascades dropped).
type work struct {
	canon   map[string][]format.Change
	json    map[string][]jsonEdit
	creates []newFile
	removes []removal
	moves   []fileMove
	undo    []Operation
	owners  map[string]string // the package owning each file written, by display path
	dropped []Dropped
	records []touched
	locked  []Locked
	kept    keptCase
	given   []givenValue
}

// jsonEdit is an edit of a JSON source and the offset it applies at in the current text: the
// edits of one file apply last offset first, so each pointer still names its node (FMT-02).
type jsonEdit struct {
	e      jsonsrc.Edit
	anchor int
	insert bool       // at one offset, an insertion comes after a set or a removal there
	rename *keyRename // a member's key written again in place, not an edit of jsonsrc
}

// keyRename is the key of member from of the object at pointer holder, written as to.
type keyRename struct {
	holder, from, to string
}

// newFile is a file an operation creates (API.md N1-N5).
type newFile struct {
	display string
	content []byte
}

// fileMove is a file an operation renames, after its own edits (API.md N8); stop is the
// package directory the directories it leaves empty are removed below, "" for none.
type fileMove struct {
	from, to, stop string
	json           bool
}

// removal is a file an operation deletes; stop as for fileMove (API.md N6).
type removal struct {
	display, stop string
}

func newWork() *work {
	return &work{canon: map[string][]format.Change{}, json: map[string][]jsonEdit{}, owners: map[string]string{}}
}

// own records the package owning a file the work writes (API.md E17).
func (w *work) own(display, pkg string) {
	w.owners[display] = pkg
}

// written are the existing files the work writes, by display path, and whether each is JSON.
func (w *work) written() map[string]bool {
	out := map[string]bool{}
	for d := range w.canon { //canon:unordered builds a set
		out[d] = false
	}
	for d := range w.json { //canon:unordered builds a set
		out[d] = true
	}
	for _, m := range w.moves {
		if _, ok := out[m.from]; !ok {
			out[m.from] = m.json
		}
	}
	return out
}

// normalize judges each written file's layout once (API.md M9): one that is not a fixed point
// of its formatter is normalized entirely; true when one was, and the state must be analyzed again.
func (a *applier) normalize(files map[string]bool) (bool, error) {
	again := false
	for _, d := range slices.Sorted(maps.Keys(files)) {
		s, err := a.state(d)
		if err != nil {
			return false, err
		}
		if s.checked || !s.existed {
			continue
		}
		s.checked = true
		out, err := a.canonical(d, s.cur, files[d])
		if err != nil {
			return false, err
		}
		if !bytes.Equal(out, s.cur) {
			s.wrote(out, nil)
			s.normalized, a.dirty, again = true, true, true
		}
	}
	a.stamp()
	return again, nil
}

// canonical is raw in canonical layout, judged on the raw bytes: FileSet folds CRLF, so a raw
// carriage return is itself a layout to normalize (log-2026-09-29 M4, M9 on raw bytes).
func (a *applier) canonical(display string, raw []byte, isJSON bool) ([]byte, error) {
	var fs source.FileSet
	src, err := fs.Add(display, display, raw)
	if err != nil {
		return nil, err
	}
	bag := diag.NewBag(&fs, "")
	if isJSON {
		root, err := jsonsrc.Parse(src, bag)
		if err != nil {
			return nil, err
		}
		a.snap.typedNumbers(display, src.Content, root) // typed-canonical numbers (FMT-02, FORMATTER.md 14.1)
		return jsonsrc.Format(root), nil
	}
	f := a.snap.tree(display)
	if f == nil {
		return nil, errNoTree
	}
	return format.Source(src, parseKind(f), bag)
}

// parseKind is the kind a file is parsed as: a project file, or a source whose header names
// its kind (a layer, a translation), as the project reads it.
func parseKind(f *syntax.File) syntax.FileKind {
	if f.FileKind == syntax.FileProject {
		return syntax.FileProject
	}
	return syntax.FileSource
}

// unwritable refuses work that writes a file on a hidden path, or outside the project and its
// roots (an absolute display path), as a commit would and with the same reason and sentinel, so
// Apply, DryRun and Edit refuse alike (API.md N10; log-2026-09-29 M4 U5b-r, U5b-r3).
func (w *work) unwritable() error {
	names := slices.Concat(slices.Collect(maps.Keys(w.canon)), slices.Collect(maps.Keys(w.json)))
	for _, nf := range w.creates {
		names = append(names, nf.display)
	}
	for _, m := range w.moves {
		names = append(names, m.from, m.to)
	}
	for _, r := range w.removes {
		names = append(names, r.display)
	}
	slices.Sort(names)
	i := slices.IndexFunc(names, func(d string) bool { return isAbsName(d) || !visible(d) })
	switch {
	case i < 0:
		return nil
	case isAbsName(names[i]):
		return journalRefusal(ErrChanges, &fault{reasonPlace, names[i]})
	}
	return journalRefusal(ErrUnwritable, &fault{reasonHidden, names[i]})
}

// commit makes w's changes in memory: edits, then renames, deletions and creations.
func (a *applier) commit(w *work) error {
	if err := w.unwritable(); err != nil {
		return err
	}
	for _, d := range slices.Sorted(maps.Keys(w.canon)) {
		if err := a.rewriteCanon(d, w.canon[d]); err != nil {
			return err
		}
	}
	for _, d := range slices.Sorted(maps.Keys(w.json)) {
		if err := a.rewriteJSON(d, w.json[d]); err != nil {
			return err
		}
	}
	for _, m := range w.moves {
		a.move(m)
	}
	for _, r := range w.removes {
		a.files[r.display].wrote(nil, nil)
		a.leaves(r.display, r.stop)
	}
	for _, nf := range w.creates {
		s, err := a.state(nf.display)
		if err != nil {
			return err
		}
		s.wrote(nf.content, nil)
		s.checked = true
	}
	if len(w.undo) > 0 {
		a.undo = append(a.undo, w.undo)
	}
	maps.Copy(a.owners, w.owners)
	a.dropped = append(a.dropped, w.dropped...)
	a.records = append(a.records, w.records...)
	a.given = append(a.given, w.given...)
	a.locked = append(a.locked, w.locked...)
	a.kept = w.kept
	a.dirty = true
	a.stamp()
	return nil
}

func (a *applier) rewriteCanon(display string, changes []format.Change) error {
	f := a.snap.tree(display)
	s := a.files[display]
	if f == nil || s == nil || !bytes.Equal(f.Src.Content, s.cur) {
		return fmt.Errorf(fmtFile, errNoTree, display)
	}
	out, err := format.Rewrite(f, changes)
	if err != nil {
		return fmt.Errorf(fmtFileErr, display, err)
	}
	s.wrote(out, func() []region { return canonRegions(f, changes) })
	s.reprinted(func() bool {
		return !slices.ContainsFunc(changes, func(c format.Change) bool { return c.Kind != format.Replace })
	})
	return nil
}

func (a *applier) rewriteJSON(display string, edits []jsonEdit) error {
	s := a.files[display]
	order := applyOrder(edits)
	before, out := s.cur, s.cur
	var batch []jsonsrc.Edit
	var err error
	for _, e := range order {
		if e.rename == nil {
			batch = append(batch, e.e)
			continue
		}
		if out, err = rewriteBatch(out, batch); err == nil {
			out, err = renameKey(out, e.rename)
		}
		if err != nil {
			return fmt.Errorf(fmtFileErr, display, err)
		}
		batch = nil
	}
	if out, err = rewriteBatch(out, batch); err != nil {
		return fmt.Errorf(fmtFileErr, display, err)
	}
	s.wrote(out, func() []region { return jsonRegions(before, order) })
	s.reprinted(func() bool {
		return !slices.ContainsFunc(order, func(e jsonEdit) bool { return e.rename != nil || e.e.Kind != jsonsrc.Set })
	})
	return nil
}

// applyOrder is the order a JSON source's edits apply in: last offset first, so each pointer
// still names its node, and at one offset a set or a removal before an insertion (FMT-02).
func applyOrder(edits []jsonEdit) []jsonEdit {
	order := slices.Clone(edits)
	slices.Reverse(order)
	slices.SortStableFunc(order, func(x, y jsonEdit) int {
		return cmp.Or(cmp.Compare(y.anchor, x.anchor), cmp.Compare(boolRank(x.insert), boolRank(y.insert)))
	})
	return order
}

// parseJSONText is the document of the JSON text src, with the text as parsed.
func parseJSONText(src []byte) ([]byte, *jsonsrc.Node, error) {
	var fs source.FileSet
	f, err := fs.Add(jsonName, jsonName, src)
	if err != nil {
		return nil, nil, err
	}
	root, err := jsonsrc.Parse(f, diag.NewBag(&fs, ""))
	return f.Content, root, err
}

// rewriteBatch applies edits to src, none leaving it as it is.
func rewriteBatch(src []byte, edits []jsonsrc.Edit) ([]byte, error) {
	if len(edits) == 0 {
		return src, nil
	}
	return jsonsrc.Rewrite(src, edits)
}

// renameKey writes a member's key again in place, its value and every other byte kept: the
// edits inside the value, which lie after its key, are already made (API.md E11, FMT-02).
func renameKey(src []byte, rn *keyRename) ([]byte, error) {
	content, root, err := parseJSONText(src)
	if err != nil {
		return nil, err
	}
	holder := root.Find(rn.holder)
	if holder == nil {
		return nil, errNoMember
	}
	i := slices.IndexFunc(holder.Members, func(m jsonsrc.Member) bool { return m.Key == rn.from })
	if i < 0 {
		return nil, errNoMember
	}
	ks := holder.Members[i].KeySpan
	return slices.Concat(content[:ks.Start], diag.AppendJSONString(nil, rn.to), content[ks.End:]), nil
}

// move renames a file: the new one reports the change, from the old one's base bytes (N8).
func (a *applier) move(m fileMove) {
	old := a.files[m.from]
	a.leaves(m.from, m.stop)
	if back, ok := a.files[m.to]; ok && back.movedTo == m.from {
		back.cur, back.movedTo, back.normalized = old.cur, "", old.normalized // renamed back: no change
		back.steps = old.steps
		delete(a.files, m.from)
		return
	}
	nw := &fileState{display: m.to, from: m.from, raw: old.raw, cur: old.cur, checked: true, normalized: old.normalized, steps: old.steps}
	if abs, ok := a.env.Project.Abs(m.to); ok {
		nw.abs = abs
	}
	if old.from != "" {
		nw.from = old.from
	}
	old.movedTo, old.cur, old.normalized, old.steps = m.to, nil, false, nil
	a.files[m.to] = nw
}

// leaves notes the directories a file left, up to but not including the package directory stop (N6).
func (a *applier) leaves(display, stop string) {
	if stop == "" {
		return
	}
	for d := path.Dir(display); d != stop && strings.HasPrefix(d, stop+pathSep); d = path.Dir(d) {
		a.emptied[d] = stop
	}
}

// tree is the snapshot's syntax tree of the file at display, nil for none.
func (s *Snapshot) tree(display string) *syntax.File {
	for _, pkg := range s.pkgs {
		for _, f := range pkg.Files {
			if f.Src.Path == display {
				return f
			}
		}
	}
	return nil
}

// removedDirs are the directories the edit leaves empty, deepest first (API.md N6): a
// directory whose every entry is gone, or is such a directory itself.
func (a *applier) removedDirs() []Change {
	dirs := slices.Sorted(maps.Keys(a.emptied))
	slices.SortStableFunc(dirs, func(x, y string) int {
		return cmp.Compare(strings.Count(y, pathSep), strings.Count(x, pathSep))
	})
	over := a.overlay()
	gone := map[string]bool{}
	var out []Change
	for _, d := range dirs {
		abs, ok := a.env.Project.Abs(d)
		if !ok {
			continue
		}
		list, err := over.ReadDir(abs)
		if err != nil || slices.ContainsFunc(list, func(e fs.DirEntry) bool { return !gone[path.Join(d, e.Name())] }) {
			continue
		}
		gone[d] = true
		out = append(out, Change{Kind: ChangeRemovedDir, Path: d})
	}
	return out
}

// boolRank orders false before true.
func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}
