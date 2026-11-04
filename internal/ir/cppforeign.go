package ir

import (
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// defineSource is where the project writes a #define: the package, and the item as findings name it (`pkg.Enum.member` or `pkg.CONST`).
type defineSource struct{ Pkg, Origin string }

// scannedPackage is one package of the project with its parsed files.
type scannedPackage struct {
	name  string
	files []*syntax.File
}

// projectDefines are the macros the defines headers of the project write, by name, each with its first source in package-name order (CODEGEN.md §7.9, DECISIONS 403): read from declarations alone, never evaluated, for every scanned package whose first cpp emit is baked, data or types; computed once.
func (s *stage) projectDefines() map[string]defineSource {
	if s.projDefines != nil {
		return s.projDefines
	}
	s.projDefines = map[string]defineSource{}
	pkgs := s.scanned()
	slices.SortStableFunc(pkgs, func(a, b scannedPackage) int { return strings.Compare(a.name, b.name) })
	for _, sp := range pkgs {
		files := sourceOnly(sp.files)
		if !writesDefines(files) {
			continue
		}
		for _, f := range files {
			for _, d := range f.Decls {
				s.addDefines(sp.name, d)
			}
		}
	}
	return s.projDefines
}

// scanned are the packages the project scans: Input.Units, else the program's packages (a harness without a project).
func (s *stage) scanned() []scannedPackage {
	var out []scannedPackage
	if s.in.Units == nil {
		for _, cp := range s.in.Program.Packages {
			out = append(out, scannedPackage{cp.Path, cp.Files})
		}
		return out
	}
	for _, u := range s.in.Units {
		out = append(out, scannedPackage{u.Name, u.Files})
	}
	return out
}

// writesDefines reports a package whose first cpp emit, in file order, is in a mode gen/cpp writes names for: only then is a defines header written (CODEGEN.md §7.9).
func writesDefines(files []*syntax.File) bool {
	for _, f := range files {
		for _, d := range f.Decls {
			ed, ok := d.(*syntax.EmitDecl)
			if !ok || ed.Target == nil || ed.Options == nil {
				continue
			}
			if t, known := wordIndex[Target](targetWords[:], ed.Target.Name); known && t == TargetCpp {
				m := declaredMode(ed)
				return plannedMode(m) || m == ModeNone // a mode check refused writes them in every mode (DECISIONS 429)
			}
		}
	}
	return false
}

// declaredMode is the mode an emit of a code target is written with: baked unless a `mode:` option says otherwise, the last one counting as readOptions reads it.
func declaredMode(ed *syntax.EmitDecl) Mode {
	mode := ModeBaked
	for _, it := range ed.Options.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name != nil && fi.Name.Name == check.OptMode {
			mode = modeOption(fi.Value)
		}
	}
	return mode
}

// addDefines records the macros of one public declaration of pkg, by §7.9's prefix rule, unless an earlier source already wrote the name; a declaration the checker broke writes none (DECISIONS 213), and a package that was not loaded has none broken.
func (s *stage) addDefines(pkg string, d syntax.Decl) {
	switch x := d.(type) {
	case *syntax.EnumDecl:
		prefix, present := definesOf(x.Annotations)
		if !present || local(x.Mods) || s.broken(x.Name) {
			return
		}
		for _, m := range x.Members {
			s.addDefine(defineName(prefix, m.Name.Name), defineSource{pkg, pkg + qnameSep + x.Name.Name + qnameSep + m.Name.Name})
		}
	case *syntax.ConstDecl:
		prefix, present := definesOf(x.Annotations)
		if present && !local(x.Mods) && !s.broken(x.Name) {
			s.addDefine(defineName(prefix, x.Name.Name), defineSource{pkg, pkg + qnameSep + x.Name.Name})
		}
	}
}

// broken reports a declaration whose name the checker marked broken, as assembleDecl and declaredOnly drop it.
func (s *stage) broken(name *syntax.Ident) bool { return s.info.Broken[s.info.Defs[name]] }

func (s *stage) addDefine(name string, src defineSource) {
	if _, seen := s.projDefines[name]; !seen && defineProblem(name) == nil { // a name E8011 refuses is no define: its finding is the defining package's
		s.projDefines[name] = src
	}
}

// declaredName is a name of a C++ plan with its qualified origin and the item it was declared for, nil for a fixed name; id marks a table id (E8005 foreignId), note how a foreign define's finding names it otherwise.
type declaredName struct {
	name, origin string
	item         any
	id           bool
	note         foreignNote
}

// hint is the origin d's foreign-define finding names.
func (d declaredName) hint() string {
	if d.note.origin != "" {
		return d.note.origin
	}
	return d.origin
}

// declaredNames are the names pl declares in every scope, each scope's in name order, then the names it shares in its namespaces and the segments of the namespace (CODEGEN.md §3.5), each (name, origin, way) once: a shared entry is skipped when a scope declared the same (name, origin), and is fixed only when it is a segment of the namespace. A scope's origin that already starts with a package's name (an inherited getter of another package's record) is kept as written.
func (s *stage) declaredNames(pl *CppNamePlan) []declaredName {
	var out []declaredName
	seen, scoped := map[declaredKey]bool{}, map[declaredPair]bool{}
	add := func(d declaredName) {
		if k := d.key(); !seen[k] {
			seen[k] = true
			out = append(out, d)
		}
	}
	for _, sc := range pl.scopes {
		for _, name := range slices.Sorted(maps.Keys(sc.names)) {
			d := declaredName{name, s.scopeOrigin(pl, sc.names[name]), sc.items[name], sc.ids, sc.notes[name]}
			scoped[declaredPair{d.name, d.origin}] = true
			add(d)
		}
	}
	for _, n := range pl.shared {
		if !scoped[declaredPair{n.name, n.origin}] {
			add(declaredName{name: n.name, origin: n.origin, item: n.item, note: foreignNote{fixed: n.meets == meetsNamespace}})
		}
	}
	return out
}

// declaredKey is what makes two declared names one finding: the name, its origin as declared and the way its finding is told, so that a name met in two ways is two findings (CODEGEN.md §7.9, DECISIONS 442).
type declaredKey struct {
	name, origin       string
	id, derived, fixed bool
}

// declaredPair is a name and its origin, whatever way its finding is told.
type declaredPair struct{ name, origin string }

// key is d's finding: its name and origin as declared, and how its way out is told.
func (d declaredName) key() declaredKey {
	return declaredKey{d.name, d.origin, d.id, d.note.derived, d.note.fixed}
}

// scopeOrigin is a scope origin as messages name it: as written when it starts with the name of a package, else qualified by pl's.
// The lookup cannot misname one: a scope origin is pl's own qualified name, another loaded package's (inherited getter, reader, define ref) or one segment, never looked up (E8005_85).
func (s *stage) scopeOrigin(pl *CppNamePlan, origin string) string {
	segs := strings.Split(origin, qnameSep)
	for i := len(segs) - 1; i >= 1; i-- {
		if s.units[strings.Join(segs[:i], qnameSep)] != nil {
			return origin
		}
	}
	return pl.qualified(origin)
}

// declareFixed is declare in sc for a name the generator writes whatever the declarations say (E8005 foreignFixed).
func (pl *CppNamePlan) declareFixed(sc *nameScope, name, origin string, item any) {
	pl.noteDeclared(sc, name, foreignNote{fixed: true}, func() { pl.declare(sc, name, origin, item) })
}

// declareOwn is declare in sc for a name built on the Canon name of item, which @cpp(name:) does not change (E8005 foreignDerived).
func (pl *CppNamePlan) declareOwn(sc *nameScope, name, origin string, item any) {
	pl.noteDeclared(sc, name, foreignNote{derived: true}, func() { pl.declare(sc, name, origin, item) })
}

// innerOwn is innerFrom for a name built on the Canon name of m's fn and on its class's, whose @cpp(name:) renames it (E8005 foreignDerived; `derived` beside another name of the same class only).
func (pl *CppNamePlan) innerOwn(sc *nameScope, m cppMethod, meets cppMeet, build func() string) {
	pl.noteDeclared(sc, build(), foreignNote{derived: true, owner: m.class}, func() { pl.innerFrom(sc, m, meets, build) })
}

// innerFixed is localInner for a name the generator writes whatever the declarations say (E8005 foreignFixed).
func (pl *CppNamePlan) innerFixed(sc *nameScope, name, origin string, meets cppMeet) {
	pl.noteDeclared(sc, name, foreignNote{fixed: true}, func() { pl.localInner(sc, name, origin, nil, meets) })
}
