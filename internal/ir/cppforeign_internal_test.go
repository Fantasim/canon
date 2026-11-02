package ir

import (
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	foreignEmitData     = "emit cpp { out: \"@features/p\", mode: data }\n"
	foreignEmitDefault  = "emit cpp { out: \"@features/p\" }\n"
	foreignEmitEmbedded = "emit cpp { out: \"@features/p\", mode: embedded }\n"
	foreignEmitTypes    = "emit cpp { out: \"@features/p\", mode: types }\n"
	foreignEmitGo       = "emit go { out: \"@features/p\", package: \"p\" }\n"
	foreignKinds        = "package p\n\n/// Kinds.\nenum Kind @cpp(defines: \"K_\") {\n  /// One.\n  A\n  /// Two.\n  K_B\n  /// Gone.\n  retired C\n}\n\n"
	foreignLimit        = "/// A limit.\n@cpp(defines: \"\")\nconst MAX = 3\n\n"
)

// foreignPkg is one package's single source file for TestProjectDefines.
type foreignPkg struct{ name, src string }

// foreignFiles parses each package's source as a project would, in the order given.
func foreignFiles(t *testing.T, pkgs []foreignPkg) map[string][]*syntax.File {
	t.Helper()
	fs := &source.FileSet{}
	bag := diag.NewBag(fs, "")
	out := map[string][]*syntax.File{}
	for _, p := range pkgs {
		src, err := fs.Add(p.name+"/"+p.name+".canon", "/"+p.name+".canon", []byte(p.src))
		if err != nil {
			t.Fatal(err)
		}
		out[p.name] = append(out[p.name], syntax.Parse(src, syntax.FileSource, bag))
	}
	return out
}

// foreignStage is a stage over the packages pkgs: their units when units is set (every scanned package), else the program's (the ir harness, Units nil).
func foreignStage(t *testing.T, pkgs []foreignPkg, units bool) *stage {
	t.Helper()
	files := foreignFiles(t, pkgs)
	var in Input
	in.Program = &check.Program{}
	for _, p := range pkgs {
		if units {
			in.Units = append(in.Units, &project.Unit{Name: p.name, Files: files[p.name]})
		} else {
			in.Program.Packages = append(in.Program.Packages, &check.Package{Path: p.name, Files: files[p.name]})
		}
	}
	return &stage{in: in, info: &check.Info{}, units: map[string]*unit{"p": {}, "q": {}}}
}

// CODEGEN.md §7.9, DECISIONS 403: the project's defines come from declarations alone, by the prefix rule of the defines header; only a package whose first cpp emit is baked, data or types writes any; a local declaration writes none; a name keeps its first source in package-name order; Units nil reads the program's packages.
func TestProjectDefines(t *testing.T) {
	kind := defineSource{Pkg: "p", Origin: "p.Kind.A"}
	for _, c := range []struct {
		name  string
		pkgs  []foreignPkg
		units bool
		want  map[string]defineSource
	}{
		{"prefix rule, retired member included", []foreignPkg{{"p", foreignKinds + foreignEmitDefault}}, true, map[string]defineSource{
			"K_A": kind, "K_B": {"p", "p.Kind.K_B"}, "K_C": {"p", "p.Kind.C"},
		}},
		{"constants", []foreignPkg{{"p", "package p\n\n" + foreignLimit + foreignEmitData}}, true, map[string]defineSource{"MAX": {"p", "p.MAX"}}},
		{"a constant without defines", []foreignPkg{{"p", "package p\n\n/// A limit.\nconst MAX = 3\n\n" + foreignEmitData}}, true, map[string]defineSource{}},
		{"local enum and local constant", []foreignPkg{{"p", "package p\n\n/// Kinds.\nlocal enum Kind @cpp(defines: \"K_\") {\n  /// One.\n  A\n}\n\n/// A limit.\n@cpp(defines: \"\")\nlocal const MAX = 3\n\n" + foreignEmitData}}, true, map[string]defineSource{}},
		{"types mode", []foreignPkg{{"p", foreignKinds + foreignEmitTypes}}, true, map[string]defineSource{"K_A": kind, "K_B": {"p", "p.Kind.K_B"}, "K_C": {"p", "p.Kind.C"}}},
		{"embedded writes none", []foreignPkg{{"p", foreignKinds + foreignEmitEmbedded}}, true, map[string]defineSource{}},
		{"the first cpp emit decides", []foreignPkg{{"p", foreignKinds + foreignEmitEmbedded + foreignEmitData}}, true, map[string]defineSource{}},
		{"no cpp emit", []foreignPkg{{"p", foreignKinds + foreignEmitGo}}, true, map[string]defineSource{}},
		{"no emit", []foreignPkg{{"p", foreignKinds}}, true, map[string]defineSource{}},
		{"first source in package-name order", []foreignPkg{
			{"q", "package q\n\n/// Kinds.\nenum Kind @cpp(defines: \"\") {\n  /// One.\n  X\n}\n\n" + foreignEmitData},
			{"p", "package p\n\n/// Kinds.\nenum Kind @cpp(defines: \"\") {\n  /// One.\n  X\n}\n\n" + foreignEmitData},
		}, true, map[string]defineSource{"X": {"p", "p.Kind.X"}}},
		{"Units nil reads the program's packages", []foreignPkg{{"p", foreignKinds + foreignEmitDefault}}, false, map[string]defineSource{
			"K_A": kind, "K_B": {"p", "p.Kind.K_B"}, "K_C": {"p", "p.Kind.C"},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := foreignStage(t, c.pkgs, c.units)
			got := s.projectDefines()
			if !maps.Equal(got, c.want) {
				t.Errorf("project defines %v, want %v", got, c.want)
			}
			if again := s.projectDefines(); !maps.Equal(again, got) {
				t.Errorf("second call %v, want the first's %v", again, got)
			}
		})
	}
}

// CODEGEN.md §7.9, DECISIONS 391, 403: the guard of a package is its own defines and those of the packages it reads, sorted and distinct: the set gen/cpp wraps and the set the foreign-define check excepts.
func TestCppGuardNames(t *testing.T) {
	own := &Enum{Pkg: "p", Name: "Kind", CppDefines: "P_", HasCppDefines: true, Members: []*EnumMember{{Name: "B"}, {Name: "A"}}}
	p := &Package{
		Name:  "p",
		Types: []Type{own, &Enum{Pkg: "p", Name: "Plain", Members: []*EnumMember{{Name: "Z"}}}},
		Consts: []*Const{
			{Name: "MAX", CppDefines: "", HasCppDefines: true},
		},
		Imports: []*PackageRef{
			{Name: "q", CppDefines: []string{"Q_A", "Q_B"}},
			{Name: "r", CppDefines: []string{"Q_B", "R_A"}},
			{Name: "s"},
		},
	}
	set := map[string]bool{}
	for _, n := range CppDefineNames(p) {
		set[n] = true
	}
	for _, ref := range p.Imports {
		for _, n := range ref.CppDefines {
			set[n] = true
		}
	}
	want := slices.Sorted(maps.Keys(set))
	if got := CppGuardNames(p); !slices.Equal(got, want) {
		t.Errorf("CppGuardNames = %v, want gen/cpp's set %v", got, want)
	}
	if got := CppGuardNames(&Package{Name: "p"}); len(got) != 0 {
		t.Errorf("a package with no defines and no imports guards %v", got)
	}
}

// CODEGEN.md §3.5, §7.9, DECISIONS 403, 442: a scope origin that starts with a package's name (an inherited getter of q's record) is named as written, one without is qualified by the plan's package, and a name declared in a scope and shared in the namespace is one finding: one per (name, origin, way), so a table id named like its table's container is two, and one (name, origin) met in two ways (declared in one scope, derived in another) is two; each name of a baked table's id enum is a table id, a scope's notes say how a finding names the rest (ERRORS.md E8005 foreignId, foreignDerived, foreignFixed), and a shared name no scope declares is fixed only for a namespace segment.
func TestDeclaredNames(t *testing.T) {
	pl := &CppNamePlan{namer: newNamer("p", cppValidIdent)}
	item, row, table := &Const{Name: "C"}, &Record{Name: "Rec"}, &Value{Name: "recs"}
	derived, byRecord, fixed := foreignNote{derived: true}, foreignNote{origin: "p.Rec"}, foreignNote{fixed: true}
	scope := pl.scope("p")
	scope.names = map[string]string{"C": "C", "GetF": "q.Rec.f", "Tone": "p.Tone", "Row": "q.Rec", "Recs": "p.recs", "RecId": "p.recs", "Both": "p.both"}
	scope.items = map[string]any{"C": item, "Row": row, "Recs": table, "RecId": table}
	scope.notes = map[string]foreignNote{"Recs": derived, "RecId": byRecord}
	twice := pl.scope("twice")
	twice.names, twice.notes = map[string]string{"Both": "p.both", "Fix": "p"}, map[string]foreignNote{"Both": derived, "Fix": fixed}
	ids := pl.scope("RecId")
	ids.ids, ids.names, ids.items = true, map[string]string{"Recs": "p.recs.Recs"}, map[string]any{"Recs": table}
	pl.shared = []cppShared{{name: "C", origin: "p.C", item: item}, {name: "Recs", origin: "p.recs", item: table}, {name: "RecId", origin: "p.recs", item: table}, {name: "p", origin: "p", meets: meetsNamespace}, {name: "Z", origin: "p.Z", meets: meetsLocal}, {name: "Fix", origin: "p", meets: meetsLocal}}
	got := foreignStage(t, nil, true).declaredNames(pl)
	want := []declaredName{
		{name: "Both", origin: "p.both"}, {name: "C", origin: "p.C", item: item}, {name: "GetF", origin: "q.Rec.f"}, {name: "RecId", origin: "p.recs", item: table, note: byRecord},
		{name: "Recs", origin: "p.recs", item: table, note: derived}, {name: "Row", origin: "q.Rec", item: row}, {name: "Tone", origin: "p.Tone"},
		{name: "Both", origin: "p.both", note: derived}, {name: "Fix", origin: "p", note: fixed}, {name: "Recs", origin: "p.recs.Recs", item: table, id: true},
		{name: "p", origin: "p", note: fixed}, {name: "Z", origin: "p.Z"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("declared names %v, want %v", got, want)
	}
}
