package edit

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// nameTarget is the declaration a RenameName names (API.md E27): its object, its package (nil
// for a built-in), and the function declaring it when it is a parameter or local.
type nameTarget struct {
	obj check.Object
	pkg *check.Package
	fn  check.Object
}

// namePos is a name given as `file:line:col`.
type namePos struct {
	file      string
	line, col int
}

// lookupName resolves name, `[package:]word{.word}` or `file:line:col` (API.md E27).
func (s *Snapshot) lookupName(name string) (nameTarget, error) {
	if p, ok := splitPosition(name); ok {
		return s.nameAt(p)
	}
	pkg, words, ok := splitNames(name)
	if !ok {
		return nameTarget{}, &NameError{Err: ErrBadPath}
	}
	t, err := s.nameRoot(pkg, words[0])
	for _, w := range words[1:] {
		if err != nil {
			break
		}
		t, err = s.nameMember(t, w)
	}
	return t, err
}

// splitPosition reads `file:line:col`, the line and the column positive decimals.
func splitPosition(name string) (namePos, bool) {
	rest, colText, okCol := cutLast(name)
	file, lineText, okLine := cutLast(rest)
	line, lineOK := positive(lineText)
	col, colOK := positive(colText)
	return namePos{file: file, line: line, col: col}, okCol && okLine && lineOK && colOK && file != ""
}

// cutLast splits s at its last package mark.
func cutLast(s string) (string, string, bool) {
	i := strings.LastIndex(s, packageMark)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(packageMark):], true
}

// positive is s as a decimal above 0, digits only.
func positive(s string) (int, bool) {
	for i := range len(s) {
		if !isDigit(s[i]) {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n > 0
}

// splitNames reads `[package:]word{.word}`.
func splitNames(name string) (string, []string, bool) {
	pkg, rest, prefixed := strings.Cut(name, packageMark)
	if !prefixed {
		pkg, rest = "", name
	}
	words := strings.Split(rest, dotSeg)
	return pkg, words, (!prefixed || allWords(strings.Split(pkg, dotSeg))) && allWords(words)
}

func allWords(ws []string) bool {
	return !slices.ContainsFunc(ws, func(w string) bool { return !isWord(w) })
}

// nameRoot is the top-level type, function, let or const word names: any of package pkg, else a
// public one of every package; none is ErrNoPath, several ErrAmbiguousPath (API.md E27).
func (s *Snapshot) nameRoot(pkg, word string) (nameTarget, error) {
	var found []check.Object
	for _, p := range s.pkgs {
		if pkg != "" && p.Path != pkg {
			continue
		}
		for _, obj := range p.Decls {
			if obj.Name() == word && topName(obj) && (pkg != "" || public(obj)) {
				found = append(found, obj)
			}
		}
	}
	switch len(found) {
	case 0:
		return nameTarget{}, &NameError{Err: ErrNoPath}
	case 1:
		return s.targetOf(found[0]), nil
	}
	names := make([]string, len(found))
	for i, obj := range found {
		names[i] = obj.Pkg() + packageMark + obj.Name()
	}
	slices.Sort(names)
	return nameTarget{}, &NameError{Err: ErrAmbiguousPath, Detail: strings.Join(names, listSep), Candidates: names}
}

// topName reports a declaration a name's first word may name (API.md E27).
func topName(obj check.Object) bool {
	switch obj.Kind() {
	case check.ObjTypeName, check.ObjFn, check.ObjLet, check.ObjConst:
		return true
	default:
		return false
	}
}

// nameMember is word w after t: a field or method of a record, a case of a variant then a field
// or method of it, a member of an enum, a parameter or local of a function (API.md E27).
func (s *Snapshot) nameMember(t nameTarget, w string) (nameTarget, error) {
	switch t.obj.Kind() {
	case check.ObjTypeName:
		return s.itemNamed(declItems(typeDecl(t.obj.Type())), w)
	case check.ObjCase:
		return s.itemNamed(declItems(t.obj.Decl()), w)
	case check.ObjFn, check.ObjMethod:
		return s.localNamed(t.obj, w)
	case check.ObjLet, check.ObjConst:
		return nameTarget{}, keyRefusal(t.obj.Type())
	default:
		return nameTarget{}, &NameError{Err: ErrNoPath}
	}
}

// typeDecl is the declaration of the record, variant or enum t is, aliases followed.
func typeDecl(t types.Type) syntax.Node {
	if t == nil {
		return nil
	}
	switch b := t.Base().(type) {
	case *types.RecordType:
		return b.Decl
	case *types.AppliedRecord:
		return b.Rec.Decl
	case *types.VariantType:
		return b.Decl
	case *types.EnumType:
		return b.Decl
	}
	return nil
}

// declItems are the items of a record, variant, case or enum declaration.
func declItems(n syntax.Node) []syntax.Node {
	var out []syntax.Node
	switch d := n.(type) {
	case *syntax.RecordDecl:
		if d != nil && d.Body != nil {
			out = nodesOf(d.Body.Items)
		}
	case *syntax.VariantCase:
		if d != nil && d.Body != nil {
			out = nodesOf(d.Body.Items)
		}
	case *syntax.VariantDecl:
		if d != nil {
			out = nodesOf(d.Items)
		}
	case *syntax.EnumDecl:
		if d != nil {
			out = nodesOf(d.Members)
		}
	}
	return out
}

// nodesOf is a list of items as nodes.
func nodesOf[T syntax.Node](items []T) []syntax.Node {
	out := make([]syntax.Node, len(items))
	for i, it := range items {
		out[i] = it
	}
	return out
}

// itemNamed is the field, method, case or member among items named w.
func (s *Snapshot) itemNamed(items []syntax.Node, w string) (nameTarget, error) {
	for _, it := range items {
		if id := itemName(it); id != nil && id.Name == w && s.info.Defs[id] != nil {
			return s.targetOf(s.info.Defs[id]), nil
		}
	}
	return nameTarget{}, &NameError{Err: ErrNoPath}
}

// itemName is the declaring name of a field, method, case or member, nil for another item.
func itemName(n syntax.Node) *syntax.Ident {
	switch d := n.(type) {
	case *syntax.FieldDecl:
		return d.Name
	case *syntax.FnDecl:
		return d.Name
	case *syntax.VariantCase:
		return d.Name
	case *syntax.EnumMember:
		return d.Name
	case *syntax.RecordDecl:
		return d.Name
	case *syntax.VariantDecl:
		return d.Name
	case *syntax.EnumDecl:
		return d.Name
	}
	return nil
}

// localNamed is the parameter or local w of function fn; several declared under that name are
// ErrAmbiguousPath listing their positions (API.md E27).
func (s *Snapshot) localNamed(fn check.Object, w string) (nameTarget, error) {
	objs, ids := s.locals(fn, w)
	switch len(objs) {
	case 0:
		return nameTarget{}, &NameError{Err: ErrNoPath}
	case 1:
		return s.targetOf(objs[0]), nil
	}
	at := make([]string, len(ids))
	for i, id := range ids {
		at[i] = namePlace(fn.File(), id)
	}
	return nameTarget{}, &NameError{Err: ErrAmbiguousPath, Detail: strings.Join(at, listSep), Candidates: at}
}

// locals are the parameters and locals fn declares named w, in source order, with their names.
func (s *Snapshot) locals(fn check.Object, w string) ([]check.Object, []*syntax.Ident) {
	var objs []check.Object
	var ids []*syntax.Ident
	if fn.Decl() == nil {
		return nil, nil
	}
	syntax.Inspect(fn.Decl(), func(n syntax.Node) bool {
		id, ok := n.(*syntax.Ident)
		if !ok || id.Name != w {
			return n != nil
		}
		if o := s.info.Defs[id]; o != nil && isLocal(o) && !slices.Contains(objs, o) {
			objs, ids = append(objs, o), append(ids, id)
		}
		return true
	})
	return objs, ids
}

// isLocal reports a parameter or a local.
func isLocal(o check.Object) bool {
	return o.Kind() == check.ObjParam || o.Kind() == check.ObjLocal
}

// namePlace is n's place as `file:line:col` (API.md 1.3).
func namePlace(f *syntax.File, n syntax.Node) string {
	line, col := f.Src.Position(f.Span(n).Start)
	return fmt.Sprintf(fmtPosition, f.Src.Path, line, col)
}

// nameAt is the declaration named by the identifier the checker resolved at p (API.md E27).
func (s *Snapshot) nameAt(p namePos) (nameTarget, error) {
	f := s.tree(p.file)
	if f == nil {
		return nameTarget{}, &NameError{Err: ErrNoPath}
	}
	var obj check.Object
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil || obj != nil {
			return false
		}
		if o := s.info.ObjectOf(n); o != nil && covers(f, n, p) {
			obj = o
		}
		return obj == nil
	})
	if obj == nil {
		return nameTarget{}, &NameError{Err: ErrNoPath}
	}
	return s.targetOf(obj), nil
}

// covers reports an identifier n holding the byte at p's line and column.
func covers(f *syntax.File, n syntax.Node, p namePos) bool {
	sp := f.Span(n)
	line, col := f.Src.Position(sp.Start)
	return line == p.line && col <= p.col && p.col < col+sp.Len()
}

// targetOf is obj with its package and, for a parameter or local, its function.
func (s *Snapshot) targetOf(obj check.Object) nameTarget {
	t := nameTarget{obj: obj, pkg: s.byPath[obj.Pkg()]}
	if isLocal(obj) {
		chain := holders(obj.File(), obj.Decl())
		if i := slices.IndexFunc(chain, isFn); i >= 0 {
			t.fn = s.info.Defs[itemName(chain[i])]
		}
	}
	return t
}

func isFn(n syntax.Node) bool {
	_, ok := n.(*syntax.FnDecl)
	return ok
}

// holders are the records, variants, enums, cases and functions of f holding n, outermost first.
func holders(f *syntax.File, n syntax.Node) []syntax.Node {
	if f == nil || n == nil {
		return nil
	}
	sp := f.Span(n)
	var out []syntax.Node
	syntax.Inspect(f, func(m syntax.Node) bool {
		if m == nil || m == n || !contains(f.Span(m), sp) {
			return false
		}
		if itemName(m) != nil {
			if _, isField := m.(*syntax.FieldDecl); !isField {
				out = append(out, m)
			}
		}
		return true
	})
	return out
}
