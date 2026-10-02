package edit

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// declSite is a declaration by the place of its name among the declaring names of its file,
// which an analysis of the file normalized finds again (API.md M9).
type declSite struct {
	file    string
	ordinal int
}

// renameSite resolves and judges a RenameName in E21's order: name (E27), kind (E28, E29), new
// name (E30), base (E32). The site of its target is nil when the new name is the old one.
func (s *Snapshot) renameSite(op Operation) (*declSite, error) {
	t, err := s.lookupName(op.Path)
	if err != nil {
		return nil, err
	}
	if err := s.renamable(t); err != nil {
		return nil, err
	}
	decl, ok := s.declOcc(t.obj)
	if !ok {
		return nil, &NameError{Err: ErrBadOp, Detail: detailNotRenamable}
	}
	if err := checkNewName(op.Name); err != nil || op.Name == t.obj.Name() {
		return nil, err
	}
	if broken := s.brokenAt(t); len(broken) > 0 {
		return nil, &NotEditableError{Reason: ReasonBroken, Refs: broken}
	}
	site := s.siteOf(decl)
	return &site, nil
}

// renamable refuses what API.md E28 and E29 do not let a RenameName rename: data (an entry key,
// an enum member, a variant case), what canon.lock names, and any other declaration.
func (s *Snapshot) renamable(t nameTarget) error {
	if t.pkg == nil || t.obj.Decl() == nil {
		return &NameError{Err: ErrBadOp, Detail: detailNotRenamable}
	}
	switch t.obj.Kind() {
	case check.ObjConst, check.ObjFn, check.ObjMethod, check.ObjParam, check.ObjLocal:
		return nil
	case check.ObjLet:
		return stableRefusal(stableTable(t.obj.Type()))
	case check.ObjTypeName:
		e, isEnum := t.obj.Type().(*types.EnumType)
		return stableRefusal(isEnum && e.Codes != nil)
	case check.ObjField:
		f := s.fieldOf(t.obj)
		if f == nil {
			return &NameError{Err: ErrBadOp, Detail: detailNotRenamable}
		}
		return stableRefusal(f.Stable)
	case check.ObjMember:
		e, isEnum := t.obj.Type().(*types.EnumType)
		return dataRefusal(isEnum && e.Codes != nil, detailFixedData)
	case check.ObjEntry:
		return dataRefusal(stableTable(s.entryTable(t.obj)), detailData)
	case check.ObjCase:
		return dataRefusal(false, detailFixedData)
	default:
		return &NameError{Err: ErrBadOp, Detail: detailNotRenamable}
	}
}

// stableRefusal is ErrStableKey for what canon.lock names (API.md E29), nil otherwise.
func stableRefusal(stable bool) error {
	if stable {
		return &NameError{Err: ErrStableKey, Detail: detailLocked}
	}
	return nil
}

// dataRefusal is the refusal of a name that names data (API.md E28): ErrStableKey when stable
// (E4), else ErrBadOp with detail, which names Rename for a key and says an enum member or a
// variant case cannot be renamed (DECISIONS 277).
func dataRefusal(stable bool, detail string) error {
	if stable {
		return &NameError{Err: ErrStableKey, Detail: detailStableData}
	}
	return &NameError{Err: ErrBadOp, Detail: detail}
}

// keyRefusal is the refusal of a key after a let or const: data when it is a table, a keyed
// list or a map (API.md E28), else nothing at that name.
func keyRefusal(t types.Type) error {
	if t == nil {
		return &NameError{Err: ErrNoPath}
	}
	switch b := t.Base().(type) {
	case *types.TableType:
		return dataRefusal(b.Stable, detailData)
	case *types.MapType:
		return dataRefusal(false, detailData)
	case *types.ListType:
		if b.KeyedBy != nil {
			return dataRefusal(false, detailData)
		}
	}
	return &NameError{Err: ErrNoPath}
}

// stableTable reports a stable table type.
func stableTable(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Base().(*types.TableType)
	return ok && b.Stable
}

// entryTable is the type of the let an entry key belongs to: its `entry` line's, or the let
// whose literal holds it; nil when none.
func (s *Snapshot) entryTable(obj check.Object) types.Type {
	if d, ok := obj.Decl().(*syntax.EntryDecl); ok {
		if let := s.info.NameUses[d.Table]; let != nil {
			return let.Type()
		}
		return nil
	}
	f := obj.File()
	sp := f.Span(obj.Decl())
	for _, d := range f.Decls {
		if let, isLet := d.(*syntax.LetDecl); isLet && contains(f.Span(let), sp) && s.info.Defs[let.Name] != nil {
			return s.info.Defs[let.Name].Type()
		}
	}
	return nil
}

// fieldOf is the field a field object declares in its record or case; nil when it has no
// declaration of a record or case (a built-in's).
func (s *Snapshot) fieldOf(obj check.Object) *types.Field {
	chain := holders(obj.File(), obj.Decl())
	if len(chain) == 0 {
		return nil
	}
	owner := s.info.Defs[itemName(chain[len(chain)-1])]
	if owner == nil || owner.Type() == nil {
		return nil
	}
	fields := fieldsOf(owner.Type())
	if i := fieldIndex(fields, obj.Name()); i >= 0 {
		return fields[i]
	}
	return nil
}

// checkNewName is API.md E30: a word, not `_`, not a reserved word.
func checkNewName(name string) error {
	if isWord(name) && syntax.LookupWord(name) == syntax.TokIdent {
		return nil
	}
	return &ValueError{Expected: expectedName, Got: strconv.Quote(name)}
}

// declOcc is the occurrence declaring obj; false when no identifier declares it.
func (s *Snapshot) declOcc(obj check.Object) (check.Occurrence, bool) {
	occs := s.prog.Occurrences(obj)
	i := slices.IndexFunc(occs, func(o check.Occurrence) bool { return o.Kind == check.OccDecl })
	if i < 0 {
		return check.Occurrence{}, false
	}
	return occs[i], true
}

// brokenAt lists, as `<file>:<line>`, what refuses a RenameName of t on its base (API.md E32):
// a broken function for a parameter or local of one, else every broken declaration, view and
// translation entry of its package and of each package importing it.
func (s *Snapshot) brokenAt(t nameTarget) []string {
	if t.fn != nil {
		if s.info.Broken[t.fn] {
			return []string{lineOf(t.fn.File(), t.fn.Decl())}
		}
		return nil
	}
	var out []string
	for _, p := range s.pkgs {
		if p == t.pkg || slices.Contains(p.Imports, t.pkg) {
			out = append(out, s.brokenIn(p)...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// brokenIn lists the broken declarations, views and translation entries of p.
func (s *Snapshot) brokenIn(p *check.Package) []string {
	var out []string
	for _, obj := range p.Decls {
		if s.info.Broken[obj] && obj.File() != nil {
			out = append(out, lineOf(obj.File(), obj.Decl()))
		}
	}
	for _, f := range p.Files {
		out = append(out, s.brokenFile(f)...)
	}
	return out
}

// brokenFile lists the declarations of f the parser dropped, its broken views and translations.
func (s *Snapshot) brokenFile(f *syntax.File) []string {
	var out []string
	for _, d := range f.Decls {
		bad, isBad := d.(*syntax.BadDecl)
		view, isView := d.(*syntax.ViewDecl)
		switch {
		case isBad:
			out = append(out, lineOf(f, bad))
		case isView && check.ViewBroken(s.info, view):
			out = append(out, lineOf(f, view))
		}
	}
	for _, e := range f.Entries {
		if s.info.BrokenTranslations[e] {
			out = append(out, lineOf(f, e))
		}
	}
	return out
}

// lineOf is n's place as `<file>:<line>` (API.md E32).
func lineOf(f *syntax.File, n syntax.Node) string {
	line, _ := f.Src.Position(f.Span(n).Start)
	return fmt.Sprintf(fmtLine, f.Src.Path, line)
}

// siteOf is the site of the declaring identifier o.
func (s *Snapshot) siteOf(o check.Occurrence) declSite {
	site := declSite{file: o.File.Src.Path, ordinal: -1}
	n := 0
	s.eachDef(o.File, func(id *syntax.Ident) bool {
		if o.File.Span(id) == o.Span {
			site.ordinal = n
			return false
		}
		n++
		return true
	})
	return site
}

// objectAt is the object the identifier at site declares, nil when there is none.
func (s *Snapshot) objectAt(site declSite) check.Object {
	f := s.tree(site.file)
	if f == nil {
		return nil
	}
	var obj check.Object
	n := 0
	s.eachDef(f, func(id *syntax.Ident) bool {
		if n == site.ordinal {
			obj = s.info.Defs[id]
			return false
		}
		n++
		return true
	})
	return obj
}

// eachDef calls yield with each declaring identifier of f, in source order, until it returns false.
func (s *Snapshot) eachDef(f *syntax.File, yield func(*syntax.Ident) bool) {
	more := true
	syntax.Inspect(f, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.Ident); ok && more && s.info.Defs[id] != nil {
			more = yield(id)
		}
		return more && n != nil
	})
}
