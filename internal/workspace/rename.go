package workspace

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/syntax"
)

// preserved is API.md E35 after the re-check of a RenameName: an occurrence of its target the
// index marks ambiguous, or an identifier of the packages re-checked that names another
// declaration than it did in before, refuses it with ErrNameClash.
func (o *EditOutcome) preserved(req EditRequest, before *build.Analysis) error {
	if len(req.Ops) != 1 || req.Ops[0].Kind != edit.OpRenameName || o.checked == nil {
		return nil
	}
	clash := o.Plan.NameClash
	if clash == "" {
		clash = captured(before.Program(), o.checked.Program(), o.rechecked, o.Plan.NameEdits)
	}
	if clash == "" {
		return nil
	}
	op := req.Ops[0]
	return &edit.OpError{Index: 0, Path: op.Path, Err: &edit.NameError{Err: edit.ErrNameClash, Detail: clash}}
}

// nameUse is an identifier, the object it declares and the other it names, nil for none.
type nameUse struct {
	file     *syntax.File
	node     syntax.Node
	def, obj check.Object
}

// nameIndex is what a program's identifiers name, by file in source order, import lists apart
// (the formatter sorts them), those the rename inserts or removes left out; and each declaration's
// key, the place of its declaring identifier among its file's, which two analyses share.
type nameIndex struct {
	uses, imports map[string][]nameUse
	keys          map[check.Object]string
	places        map[check.Object]string // each declaration's identifier as `<file>:<line>:<col>`
}

// captured is the detail of the first identifier of the files of pkgs that names in after
// another declaration than in before, or names one after naming none (a built-in compared by
// name), "" when there is none.
func captured(before, after *check.Program, pkgs []string, edits map[string]edit.NameEdit) string {
	removed, added := map[string][]int{}, map[string][]int{}
	for f, e := range edits { //canon:unordered fills two maps by file
		removed[f], added[f] = e.Removed, e.Added
	}
	b, a := indexNames(before, pkgs, removed), indexNames(after, pkgs, added)
	for _, display := range ownedFiles(before, pkgs) {
		if clash := b.inOrder(a, display); clash != "" {
			return clash
		}
		if clash := b.inImports(a, display); clash != "" {
			return clash
		}
	}
	return ""
}

// ownedFiles are the display paths of the files of pkgs in p, in byte order.
func ownedFiles(p *check.Program, pkgs []string) []string {
	var out []string
	for _, pkg := range p.Packages {
		if slices.Contains(pkgs, pkg.Path) {
			for _, f := range pkg.Files {
				out = append(out, f.Src.Path)
			}
		}
	}
	slices.Sort(out)
	return out
}

// inOrder compares the identifiers of a file outside its imports one by one, their counts equal.
func (x *nameIndex) inOrder(a *nameIndex, display string) string {
	was, now := x.uses[display], a.uses[display]
	for i := range max(len(was), len(now)) {
		switch {
		case i >= len(now):
			return clashText(placeText(was[i].file, was[i].node), nil, a)
		case i >= len(was):
			return clashText(placeText(now[i].file, now[i].node), now[i].named(), a)
		case x.useKey(was[i]) != a.useKey(now[i]):
			return clashText(placeText(was[i].file, was[i].node), now[i].named(), a)
		}
	}
	return ""
}

// inImports compares the names of a file's import lists as a set, each counted.
func (x *nameIndex) inImports(a *nameIndex, display string) string {
	left := map[string]int{}
	for _, u := range x.imports[display] {
		left[x.useKey(u)]++
	}
	for _, u := range a.imports[display] {
		k := a.useKey(u)
		if left[k] == 0 {
			return clashText(placeText(u.file, u.node), u.named(), a)
		}
		left[k]--
	}
	return ""
}

// indexNames walks once every file of pkgs in p and of the packages they import, at any depth,
// which hold every declaration their names name; skip are the identifiers left out, by file.
func indexNames(p *check.Program, pkgs []string, skip map[string][]int) *nameIndex {
	x := &nameIndex{uses: map[string][]nameUse{}, imports: map[string][]nameUse{}, keys: map[check.Object]string{}, places: map[check.Object]string{}}
	walked := map[*check.Package]bool{}
	var walk func(*check.Package)
	walk = func(pkg *check.Package) {
		if walked[pkg] {
			return
		}
		walked[pkg] = true
		for _, f := range pkg.Files {
			w := &fileWalk{x: x, info: p.Info, f: f, skip: skip[f.Src.Path]}
			syntax.Inspect(f, w.visit)
		}
		for _, q := range pkg.Imports {
			walk(q)
		}
	}
	for _, pkg := range p.Packages {
		if slices.Contains(pkgs, pkg.Path) {
			walk(pkg)
		}
	}
	return x
}

// fileWalk lists one file's identifiers into x.
type fileWalk struct {
	x          *nameIndex
	info       *check.Info
	f          *syntax.File
	defs, seen int
	skip       []int // the identifiers left out, by place among the file's
}

// visit lists every identifier in source order, resolved or not, those of import lists apart.
func (w *fileWalk) visit(n syntax.Node) bool {
	if _, isImport := n.(*syntax.Import); isImport {
		syntax.Inspect(n, func(m syntax.Node) bool {
			w.ident(m, w.x.imports)
			return m != nil
		})
		return false
	}
	w.ident(n, w.x.uses)
	return n != nil
}

// ident lists n into to when it is an identifier the rename keeps, keying a declaration it declares.
func (w *fileWalk) ident(n syntax.Node, to map[string][]nameUse) {
	switch n.(type) {
	case *syntax.Ident, *syntax.IdentExpr:
		w.seen++
		if slices.Contains(w.skip, w.seen-1) {
			return
		}
	default:
		return
	}
	var def, obj check.Object
	switch id := n.(type) {
	case *syntax.Ident:
		def, obj = w.info.Defs[id], w.info.NameUses[id]
		if def != nil {
			w.x.keys[def] = w.f.Src.Path + keySep + strconv.Itoa(w.defs)
			w.x.places[def] = placeText(w.f, id)
			w.defs++
		}
	case *syntax.IdentExpr:
		obj = w.info.Uses[id]
	default:
		return
	}
	if obj == def {
		obj = nil
	}
	to[w.f.Src.Path] = append(to[w.f.Src.Path], nameUse{file: w.f, node: n, def: def, obj: obj})
}

// named is what u names: the other object, else the one it declares.
func (u nameUse) named() check.Object {
	if u.obj != nil {
		return u.obj
	}
	return u.def
}

// useKey is what u declares and names, as keys.
func (x *nameIndex) useKey(u nameUse) string {
	return x.key(u.def) + useSep + x.key(u.obj)
}

// key is o's declaration key: a built-in by its name, a declaration no identifier declares by
// its kind, package and name; "" for none.
func (x *nameIndex) key(o check.Object) string {
	if o == nil {
		return ""
	}
	if k, ok := x.keys[o]; ok {
		return k
	}
	return o.Kind().String() + keySep + o.Pkg() + keySep + o.Name()
}

// clashText names the identifier at place captured, and what it would name in a; nothing for none.
func clashText(place string, now check.Object, a *nameIndex) string {
	what := textNothing
	switch {
	case now == nil:
	case now.Pkg() == "":
		what = textBuiltin + now.Name()
	default:
		what = now.Pkg() + packageSep + now.Name()
		if at, ok := a.places[now]; ok {
			what += textDeclaredAt + at
		}
	}
	return fmt.Sprintf(fmtCaptured, place, what)
}

// placeText is n's place as `<file>:<line>:<col>` (API.md 1.3).
func placeText(f *syntax.File, n syntax.Node) string {
	line, col := f.Src.Position(f.Span(n).Start)
	return fmt.Sprintf(fmtPlace, f.Src.Path, line, col)
}
