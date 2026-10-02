package edit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// RenameRequest is API.md E31 for ops, a request that may write errors or not, under an edit
// layer or none: a RenameName is alone in a request without AllowErrors (ErrBadOp), and never
// under an edit layer (ReasonLayer); an *OpError for the first one refused.
func RenameRequest(ops []Operation, allowErrors bool, editLayer string) error {
	i := slices.IndexFunc(ops, func(op Operation) bool { return op.Kind == OpRenameName })
	switch {
	case i < 0:
		return nil
	case len(ops) > 1 || allowErrors:
		return &OpError{Index: i, Path: ops[i].Path, Err: &NameError{Err: ErrBadOp, Detail: detailAlone}}
	case editLayer != "":
		return &OpError{Index: i, Path: ops[i].Path, Err: &NotEditableError{Reason: ReasonLayer}}
	}
	return nil
}

// renameName applies a RenameName (API.md 8.9) in E21's order, after its request (E31): the
// checks of renameSite, then every occurrence of its target replaced (E33, E34) on the current
// state, planned again on the state its files' normalization leaves (M9).
func (a *applier) renameName(op Operation) error {
	site, err := a.snap.renameSite(op)
	if err != nil || site == nil {
		return err
	}
	for {
		if err := a.settle(); err != nil {
			return err
		}
		w, err := a.renameWork(*site, op.Name)
		if err != nil {
			return err
		}
		again, err := a.normalize(w.written())
		switch {
		case err != nil:
			return err
		case !again:
			return a.commit(w)
		}
	}
}

// renameWork is the work of renaming the declaration at site to newName: each occurrence the
// checker records replaced in place, a field's `@json` changed (E33, E34), each file laid out by
// the formatter after (M5); its Undo the reverse RenameName (E37).
func (a *applier) renameWork(site declSite, newName string) (*work, error) {
	obj := a.snap.objectAt(site)
	if obj == nil {
		return nil, fmt.Errorf(fmtFile, ErrInternal, site.file)
	}
	occs := a.snap.prog.Occurrences(obj)
	r := &nameRewrite{
		s: a.snap, t: a.snap.targetOf(obj), from: obj.Name(), to: newName,
		edits: map[*syntax.File][]splice{}, spans: map[source.Span]bool{}, keys: map[*syntax.TranslationEntry]bool{},
	}
	for _, o := range occs {
		r.spans[o.Span] = true
	}
	r.fieldDecl()
	done := map[source.Span]bool{}
	for _, o := range occs {
		if done[o.Span] {
			continue
		}
		done[o.Span] = true
		if err := r.occurrence(o); err != nil {
			return nil, err
		}
	}
	w := newWork()
	a.nameEdits = map[string]NameEdit{}
	for f, edits := range r.edits { //canon:unordered each file is written under its own name
		if err := a.writeWhole(w, f, edits); err != nil {
			return nil, err
		}
	}
	undo, err := r.undoName(occs, w.whole)
	if err != nil {
		return nil, err
	}
	w.undo = []Operation{{Kind: OpRenameName, Path: undo, Name: r.from}}
	a.nameClash = ambiguousOcc(occs)
	return w, nil
}

// writeWhole writes f with edits made and laid out by the formatter, and notes the identifiers
// they insert and remove, by place among the file's identifiers (API.md E35).
func (a *applier) writeWhole(w *work, f *syntax.File, edits []splice) error {
	text, added := splicedAll(f.Src.Content, edits)
	out, err := formatted(f.Src.Path, roleOf(f.Src.Path), text)
	if err != nil {
		return fmt.Errorf(fmtWrapped, ErrInternal, err)
	}
	spliced, err := parseAs(f.Src.Path, text)
	if err != nil {
		return err
	}
	var dropped []int
	for _, e := range edits {
		dropped = append(dropped, e.dropped...)
	}
	a.nameEdits[f.Src.Path] = NameEdit{Removed: identOrdinals(f, dropped), Added: identOrdinals(spliced, added)}
	w.whole[f.Src.Path] = wholeWrite{out: out, regions: itemRegions(f, edits)}
	w.own(f.Src.Path, a.snap.packageOf(f.Src.Path))
	return nil
}

// identOrdinals are the places, among f's identifiers in source order, of those starting at one
// of starts.
func identOrdinals(f *syntax.File, starts []int) []int {
	var out []int
	i := 0
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.Ident, *syntax.IdentExpr:
			if slices.Contains(starts, int(f.Span(n).Start)) {
				out = append(out, i)
			}
			i++
		}
		return n != nil
	})
	return out
}

// parseAs is content parsed as the file at display.
func parseAs(display string, content []byte) (*syntax.File, error) {
	var fs source.FileSet
	src, err := fs.Add(display, display, content)
	if err != nil {
		return nil, fmt.Errorf(fmtWrapped, ErrInternal, err)
	}
	return syntax.Parse(src, roleOf(display), diag.NewBag(&fs, "")), nil
}

// itemRegions are the top-level items of f holding an edit: the declarations, imports, amend
// blocks and translation entries the formatter may print again (API.md E33, M5, M6).
func itemRegions(f *syntax.File, edits []splice) []region {
	var out []region
	for _, n := range topItems(f) {
		sp := f.Span(n)
		if slices.ContainsFunc(edits, func(e splice) bool { return int(sp.Start) <= e.lo && e.hi <= int(sp.End) }) {
			out = append(out, region{lo: int(sp.Start), hi: int(sp.End), kind: regionNode})
		}
	}
	return out
}

// topItems are f's top-level items, in source order.
func topItems(f *syntax.File) []syntax.Node {
	return slices.Concat(nodesOf(f.Imports), nodesOf(f.Decls), nodesOf(f.Amends), nodesOf(f.Entries))
}

// nameRewrite is one RenameName's replacements, by file.
type nameRewrite struct {
	s        *Snapshot
	t        nameTarget
	from, to string
	edits    map[*syntax.File][]splice
	spans    map[source.Span]bool // the target's occurrences
	keys     map[*syntax.TranslationEntry]bool
}

func (r *nameRewrite) add(f *syntax.File, sp source.Span, text string) {
	r.edits[f] = append(r.edits[f], splice{lo: int(sp.Start), hi: int(sp.End), text: text})
}

// occurrence replaces one occurrence of the target in place (API.md E33); a translation key is
// written again whole, its kind word added or dropped (I18N.md K4).
func (r *nameRewrite) occurrence(o check.Occurrence) error {
	if o.Kind != check.OccTranslationKey {
		r.add(o.File, o.Span, r.to)
		return nil
	}
	for _, e := range o.File.Entries {
		if e.Key == nil || !contains(o.File.Span(e.Key), o.Span) {
			continue
		}
		if !r.keys[e] {
			r.keys[e] = true
			r.edits[o.File] = append(r.edits[o.File], r.keySplice(o.File, e.Key))
		}
		return nil
	}
	return fmt.Errorf(fmtFile, ErrInternal, o.File.Src.Path)
}

// keySplice writes key again with the target's segments renamed: a field or method whose name
// is a reserved segment after its kind word, any other name bare (I18N.md K4).
func (r *nameRewrite) keySplice(f *syntax.File, key *syntax.QualifiedName) splice {
	sp := f.Span(key)
	out := splice{lo: int(sp.Start), hi: int(sp.End)}
	kind := kindWords[r.t.obj.Kind()]
	var parts []string
	for i, p := range key.Parts {
		if !r.spans[f.Span(p)] {
			parts = append(parts, p.Name)
			continue
		}
		if kind != "" && i > 0 && key.Parts[i-1].Name == kind && syntax.IsReservedSegment(r.from) {
			parts = parts[:len(parts)-1]
			out.dropped = append(out.dropped, int(f.Span(key.Parts[i-1]).Start))
		}
		if kind != "" && syntax.IsReservedSegment(r.to) {
			out.added = append(out.added, len(strings.Join(append(parts, ""), dotSeg)))
			parts = append(parts, kind)
		}
		parts = append(parts, r.to)
	}
	out.text = strings.Join(parts, dotSeg)
	return out
}

// fieldDecl adds the change a renamed field's `@json` takes (API.md E34).
func (r *nameRewrite) fieldDecl() {
	fd, isField := r.t.obj.Decl().(*syntax.FieldDecl)
	f := r.t.obj.File()
	if !isField || f == nil {
		return
	}
	if field := r.s.fieldOf(r.t.obj); field != nil {
		if wire, ok := r.s.wireSplice(f, fd, field, r.to); ok {
			r.edits[f] = append(r.edits[f], wire)
		}
	}
}

// splicedAll is content with edits made, none overlapping another, and the offsets in it of
// the identifiers they insert.
func splicedAll(content []byte, edits []splice) ([]byte, []int) {
	slices.SortFunc(edits, func(x, y splice) int { return x.lo - y.lo })
	var out []byte
	var added []int
	at := 0
	for _, e := range edits {
		out = append(out, content[at:e.lo]...)
		for _, off := range e.added {
			added = append(added, len(out)+off)
		}
		out = append(out, e.text...)
		at = e.hi
	}
	return append(out, content[at:]...), added
}

// undoName is the name the Undo renames back (API.md E37): the canonical names-only path with
// the new name, or the declaration's position when that path would not name it alone (a local
// another of its function shares the new name with, or one outside any function).
func (r *nameRewrite) undoName(occs []check.Occurrence, written map[string]wholeWrite) (string, error) {
	decl := occs[slices.IndexFunc(occs, func(o check.Occurrence) bool { return o.Kind == check.OccDecl })]
	chain := holders(r.t.obj.File(), r.t.obj.Decl())
	if isLocal(r.t.obj) && (r.t.fn == nil || r.sharedName()) {
		return movedPlace(decl, written[decl.File.Src.Path].out)
	}
	words := make([]string, 0, len(chain)+1)
	for _, n := range chain {
		words = append(words, itemName(n).Name)
	}
	return r.t.obj.Pkg() + packageMark + strings.Join(append(words, r.to), dotSeg), nil
}

// sharedName reports a function declaring another parameter or local under the new name.
func (r *nameRewrite) sharedName() bool {
	objs, _ := r.s.locals(r.t.fn, r.to)
	return len(objs) > 0
}

// movedPlace is the declaration's position in its file as written: the identifier at its place
// among the file's identifiers, which a rename and the formatter keep in order (API.md E37).
func movedPlace(decl check.Occurrence, written []byte) (string, error) {
	at := identIndex(decl.File, func(id *syntax.Ident) bool { return decl.File.Span(id) == decl.Span })
	f, err := parseAs(decl.File.Src.Path, written)
	if err != nil {
		return "", err
	}
	var found *syntax.Ident
	n := 0
	identIndex(f, func(id *syntax.Ident) bool {
		found, n = id, n+1
		return n > at
	})
	if found == nil || n <= at {
		return "", fmt.Errorf(fmtFile, ErrInternal, f.Src.Path)
	}
	return namePlace(f, found), nil
}

// identIndex is the index of the first identifier of f, in source order, for which stop holds;
// -1 when none does.
func identIndex(f *syntax.File, stop func(*syntax.Ident) bool) int {
	i, at := 0, -1
	syntax.Inspect(f, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.Ident); ok && at < 0 {
			if stop(id) {
				at = i
			}
			i++
		}
		return at < 0 && n != nil
	})
	return at
}

// ambiguousOcc is the refusal detail of the first occurrence the index marks ambiguous (API.md
// E35), "" when there is none.
func ambiguousOcc(occs []check.Occurrence) string {
	for _, o := range occs {
		if o.Ambiguous {
			line, col := o.File.Src.Position(o.Span.Start)
			return fmt.Sprintf(fmtAmbiguous, o.File.Src.Path, line, col)
		}
	}
	return ""
}

// packageOf is the package owning the file at display, "" for none.
func (s *Snapshot) packageOf(display string) string {
	for _, p := range s.pkgs {
		if slices.ContainsFunc(p.Files, func(f *syntax.File) bool { return f.Src.Path == display }) {
			return p.Path
		}
	}
	return ""
}
