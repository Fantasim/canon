package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Snapshot is the checked program and settled roots of one frozen analysis (IMPLEMENTATION-PLAN §4.6).
type Snapshot struct {
	a      *build.Analysis
	prog   *check.Program
	info   *check.Info
	pkgs   []*check.Package
	byPath map[string]*check.Package
	files  map[source.FileID]*syntax.File
}

// NewSnapshot is the snapshot of a; a's packages are the ones a path may name.
func NewSnapshot(a *build.Analysis) *Snapshot {
	prog := a.Program()
	s := &Snapshot{
		a: a, prog: prog, info: prog.Info, pkgs: prog.Packages,
		byPath: map[string]*check.Package{}, files: map[source.FileID]*syntax.File{},
	}
	for _, pkg := range prog.Packages {
		s.byPath[pkg.Path] = pkg
		for _, f := range pkg.Files {
			s.files[f.Src.ID] = f
		}
	}
	return s
}

// rootRef is what a path's root names: a top-level let or const, or an enum (API.md P7a).
type rootRef struct {
	pkg  *check.Package
	obj  check.Object
	enum *types.EnumType
}

// lockName is the root as canon.lock names a collection, `package.name` (API.md E20).
func (r rootRef) lockName() string {
	return r.pkg.Path + dotSeg + r.obj.Name()
}

// at is the root as a path with no segment: what an edit keeps of it across its operations, so
// that no operation's analysis outlives the next one.
func (r rootRef) at() Path {
	return Path{Package: r.pkg.Path, Root: r.obj.Name()}
}

// qualified is the root in path form, `package:name`.
func (r rootRef) qualified() string {
	return r.pkg.Path + packageMark + r.obj.Name()
}

// lookup finds p's root: any let, const or enum of p's package (P7), else the public ones of
// every loaded package (P6); none is ErrNoPath, several ErrAmbiguousPath.
func (s *Snapshot) lookup(p Path) (rootRef, error) {
	var found []rootRef
	for _, pkg := range s.pkgs {
		if p.Package != "" && pkg.Path != p.Package {
			continue
		}
		for _, obj := range pkg.Decls {
			if r, ok := rootOf(pkg, obj, p.Root); ok && (p.Package != "" || public(obj)) {
				found = append(found, r)
			}
		}
	}
	switch len(found) {
	case 0:
		return rootRef{}, &PathError{Seg: rootSeg, Err: ErrNoPath}
	case 1:
		return found[0], nil
	}
	names := make([]string, len(found))
	for i, r := range found {
		names[i] = r.qualified()
	}
	slices.Sort(names)
	return rootRef{}, &PathError{Seg: rootSeg, Candidates: names, Err: ErrAmbiguousPath}
}

// rootOf is obj as the root name, if it is a let, a const or an enum declaration of that name.
func rootOf(pkg *check.Package, obj check.Object, name string) (rootRef, bool) {
	if obj.Name() != name {
		return rootRef{}, false
	}
	switch obj.Kind() {
	case check.ObjLet, check.ObjConst:
		return rootRef{pkg: pkg, obj: obj}, true
	case check.ObjTypeName:
		if _, isDecl := obj.Decl().(*syntax.EnumDecl); isDecl {
			e, ok := obj.Type().(*types.EnumType)
			return rootRef{pkg: pkg, obj: obj, enum: e}, ok
		}
	default:
	}
	return rootRef{}, false
}

// public reports a declaration without `local` (API.md P6, E27).
func public(obj check.Object) bool {
	var mods *syntax.Modifiers
	switch d := obj.Decl().(type) {
	case *syntax.LetDecl:
		mods = d.Mods
	case *syntax.ConstDecl:
		mods = d.Mods
	case *syntax.EnumDecl:
		mods = d.Mods
	case *syntax.RecordDecl:
		mods = d.Mods
	case *syntax.VariantDecl:
		mods = d.Mods
	case *syntax.TypeDecl:
		mods = d.Mods
	case *syntax.FnDecl:
		mods = d.Mods
	}
	return mods == nil || !mods.Local.Valid()
}

// force is r's settled value: ErrNoValue when it is poisoned (R6), ErrNotAnalyzed when the
// analysis did not select its package.
func (s *Snapshot) force(r rootRef) (value.Value, error) {
	if v, ok := s.a.Force(eval.Root{Pkg: r.pkg.Path, Name: r.obj.Name()}); ok {
		return v, nil
	}
	if s.a.Bag(r.pkg.Path) == nil {
		return nil, &PathError{Seg: rootSeg, Err: ErrNotAnalyzed}
	}
	return nil, &PathError{Seg: rootSeg, Err: ErrNoValue, Root: eval.Root{Pkg: r.pkg.Path, Name: r.obj.Name()}, At: s.a}
}

// display is the display path of a file of the snapshot, "" for none.
func (s *Snapshot) display(id source.FileID) string {
	return s.a.Files().Path(id)
}
