package check_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const viewsCase = "testdata/accept/views.txtar"

// checkedViews is the accepted views case, checked.
type checkedViews struct {
	prog  *check.Program
	files []*syntax.File
}

func loadViews(t *testing.T) checkedViews {
	t.Helper()
	cases, err := golden.Load(viewsCase)
	if err != nil || len(cases) != 1 {
		t.Fatalf("load %s: %v", viewsCase, err)
	}
	fs := &source.FileSet{}
	parse := diag.NewBag(fs, "")
	var files []*syntax.File
	for _, f := range cases[0].Archive.Files {
		src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, syntax.Parse(src, syntax.FileSource, parse))
	}
	prog := check.Check(context.Background(), exampleProject(), files, check.Bags{}, literalFolder{})
	return checkedViews{prog: prog, files: files}
}

// view is the view whose target is written as target ("Kind.weapon"), nil when none.
func (v checkedViews) view(target string) *syntax.ViewDecl {
	for _, f := range v.files {
		for _, d := range f.Decls {
			vd, ok := d.(*syntax.ViewDecl)
			if !ok {
				continue
			}
			name := vd.Type.Name
			if vd.Case != nil {
				name += "." + vd.Case.Name
			}
			if name == target {
				return vd
			}
		}
	}
	return nil
}

// entry is the translation entry whose key is written as key.
func (v checkedViews) entry(key string) *syntax.TranslationEntry {
	for _, f := range v.files {
		for _, e := range f.Entries {
			var parts []string
			for _, p := range e.Key.Parts {
				parts = append(parts, p.Name)
			}
			if strings.Join(parts, ".") == key {
				return e
			}
		}
	}
	return nil
}

// use is the first name in value position spelled name under n.
func use(n syntax.Node, name string) *syntax.IdentExpr {
	var found *syntax.IdentExpr
	syntax.Inspect(n, func(x syntax.Node) bool {
		if id, ok := x.(*syntax.IdentExpr); ok && id.Name == name && found == nil {
			found = id
		}
		return found == nil
	})
	return found
}

// Magic names are locals of their view (a step's of its text) typed by position; only a field
// hides one (VIEWMODEL.md G7, G11, T24).
func TestMagicNames(t *testing.T) {
	v := loadViews(t)
	for _, tc := range []struct {
		rule, target, name string
		kind               check.ObjKind
		typ                string
	}{
		{"§3.4 id: a table entry", "Item", "id", check.ObjLocal, "String"},
		{"§3.4 key: a map value, the map's key type", "Level", "key", check.ObjLocal, "a.Slot"},
		{"§3.4 index: a list element", "Level", "index", check.ObjLocal, "Int"},
		{"T24 step: {index}, a local of the step text", "Item", "index", check.ObjLocal, "Int"},
		{"G11 a field named key hides the magic name", "Named", "key", check.ObjField, "String"},
		{"G11 a field named index hides the magic name", "Tier", "index", check.ObjField, "Int"},
		{"G11 a method named index does not hide it", "Rung", "index", check.ObjLocal, "Int"},
		{"§3.2 an alias of a record is its record's view", "Jewel", "cut", check.ObjField, "Int"},
		{"§3.2 an alias of a record: its positions", "Jewel", "index", check.ObjLocal, "Int"},
		{"G7 a define table's id", "names", "id", check.ObjLocal, "String"},
		{"G7 a define table's value", "names", "value", check.ObjField, "Int"},
	} {
		d := v.view(tc.target)
		id := use(d, tc.name)
		o := v.prog.Info.Uses[id]
		if id == nil || o == nil {
			t.Errorf("%s: %s not resolved in view %s", tc.rule, tc.name, tc.target)
			continue
		}
		if o.Kind() != tc.kind || v.prog.Info.Types[id].String() != tc.typ {
			t.Errorf("%s: %s is %v %v, want %v %s", tc.rule, tc.name, o.Kind(), v.prog.Info.Types[id], tc.kind, tc.typ)
		}
		if tc.kind == check.ObjLocal && o.Decl() != magicDecl(d, id) {
			t.Errorf("%s: %s is declared by %T, want its view or its step text", tc.rule, tc.name, o.Decl())
		}
	}
}

// A view of a built-in type or of an alias of one is views' E1626: check records the target and
// checks nothing inside (log-2026-09-28 U5 round 2).
func TestOtherKindTargets(t *testing.T) {
	prog, file, out := checkFile(t, "package a\n\nlocal type Money = Int\n\n"+
		"view Int {\n  title \"{nope}\"\n}\n\nview Money {\n  title \"{nope}\"\n  cost { when: nope }\n}\n")
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	for _, tc := range []struct {
		target string
		want   check.ObjKind
	}{{"Int", check.ObjBuiltin}, {"Money", check.ObjTypeName}} {
		d := checkedViews{prog: prog, files: []*syntax.File{file}}.view(tc.target)
		if o := prog.Info.NameUses[d.Type]; o == nil || o.Kind() != tc.want {
			t.Errorf("view %s: target is %v, want a %v", tc.target, o, tc.want)
		}
		if id := use(d, "nope"); prog.Info.Uses[id] != nil || prog.Info.Types[id] != nil {
			t.Errorf("view %s: its body was checked", tc.target)
		}
	}
}

// magicDecl is the node declaring the magic name id of view d: the step text holding it, else d.
func magicDecl(d *syntax.ViewDecl, id *syntax.IdentExpr) syntax.Node {
	var decl syntax.Node = d
	syntax.Inspect(d, func(n syntax.Node) bool {
		if fi, ok := n.(*syntax.FieldItem); ok && fi.Name.Name == "step" && use(fi.Value, id.Name) == id {
			decl = fi.Value
		}
		return true
	})
	return decl
}

// I18N.md T1: a translated template reads the source item's scope, the view's magic names
// included; a translated step sees an `{index}` of its own entry.
func TestTranslatedScopes(t *testing.T) {
	v := loadViews(t)
	viewKey := v.prog.Info.Uses[use(v.view("Level"), "key")]
	if got := v.prog.Info.Uses[use(v.entry("Level.title"), "key")]; got == nil || got != viewKey {
		t.Errorf("T1: Level.title's {key} is %v, want the view's %v", got, viewKey)
	}
	step := v.entry("Item.levels.step")
	o := v.prog.Info.Uses[use(step, "index")]
	if o == nil || o.Kind() != check.ObjLocal || o.Decl() != step || o.Type().String() != "Int" {
		t.Errorf("T1: Item.levels.step's {index} is %v, want an Int local of the entry", o)
	}
	for _, tc := range []struct{ key, name string }{
		{"Level.check.weak", "level"}, {"check.few", "items"}, {"Kind.weapon.title", "attack"},
		{"Level.show.avg.text", "doubled"}, {"Level.show._0.text", "heal"},
	} {
		if v.prog.Info.Uses[use(v.entry(tc.key), tc.name)] == nil {
			t.Errorf("T1: %s: %s is not resolved in its source's scope", tc.key, tc.name)
		}
	}
}

// Item names, studio names and key segments name their object (VIEWMODEL.md §3.3, T6a, G16; I18N.md K4).
func TestItemAndKeyNames(t *testing.T) {
	v := loadViews(t)
	names := func(n syntax.Node) map[string]check.ObjKind {
		out := map[string]check.ObjKind{}
		syntax.Inspect(n, func(x syntax.Node) bool {
			if id, ok := x.(*syntax.Ident); ok && v.prog.Info.NameUses[id] != nil {
				out[id.Name] = v.prog.Info.NameUses[id].Kind()
			}
			return true
		})
		return out
	}
	for _, tc := range []struct {
		rule string
		node syntax.Node
		name string
		want check.ObjKind
	}{
		{"§3.3 a field", v.view("Item"), "label", check.ObjField},
		{"§3.3 rule 3 a case field of an inline variant", v.view("Item"), "attack", check.ObjField},
		{"§3.3 rule 3 a nested field in filters", v.view("Item"), "handed", check.ObjField},
		{"G16 a studio menu", v.view("Item"), "items", check.ObjMember},
		{"§3.3 a method", v.view("Level"), "doubled", check.ObjMethod},
		{"T6a a variant's column names a case field", v.view("Kind"), "heal", check.ObjField},
		{"§3.2 a variant's case item", v.view("Kind"), "weapon", check.ObjCase},
		{"§3.2 an enum's member item", v.view("Rarity"), "rare", check.ObjMember},
		{"§3.2 a define table", v.view("names"), "names", check.ObjLet},
		{"K4 a case after its kind word", v.entry("Kind.case.food"), "food", check.ObjCase},
		{"K4 a field after its kind word", v.entry("Kind.weapon.field.attack"), "attack", check.ObjField},
		{"K4 a member after its kind word", v.entry("Rarity.member.common"), "common", check.ObjMember},
		{"K5 a record check", v.entry("Level.check.weak"), "weak", check.ObjCheck},
		{"K5 a package check", v.entry("check.few"), "few", check.ObjCheck},
		{"K5 a method", v.entry("Level.method.doubled"), "doubled", check.ObjMethod},
	} {
		if got, ok := names(tc.node)[tc.name]; !ok || got != tc.want {
			t.Errorf("%s: %s names %v, want %v", tc.rule, tc.name, got, tc.want)
		}
	}
	for _, id := range []string{"group", "main", "intro"} {
		if _, ok := names(v.entry("Item.group.main.intro"))[id]; ok {
			t.Errorf("K5: segment %s of Item.group.main.intro names an object", id)
		}
	}
}

// VIEWMODEL.md T6a, TYPES.md §3.6: a variant view's filter `kind` is the case filter; a case field stays a field.
func TestVariantCaseFilter(t *testing.T) {
	v := loadViews(t)
	want := map[string]check.ObjKind{"kind": check.ObjBuiltin, "heal": check.ObjField}
	seen := 0
	for _, it := range v.view("Kind").Items {
		fs, ok := it.(*syntax.ViewFilters)
		if !ok {
			continue
		}
		for _, f := range fs.Items {
			seen++
			if o := v.prog.Info.NameUses[f.Name]; o == nil || o.Kind() != want[f.Name.Name] {
				t.Errorf("T6a: filter %s names %v, want a %v", f.Name.Name, o, want[f.Name.Name])
			}
		}
	}
	if seen != len(want) {
		t.Errorf("T6a: view Kind filters %d names, want %d", seen, len(want))
	}
}

// VIEWMODEL.md §3.5, G16: `icon: Icon.gem`, `tone: Tone.info` name the studio's member without an import.
func TestQualifiedStudioNames(t *testing.T) {
	v := loadViews(t)
	var sels []*syntax.SelectorExpr
	syntax.Inspect(v.view("Kind"), func(n syntax.Node) bool {
		if s, ok := n.(*syntax.SelectorExpr); ok && s.X.Kind() == syntax.KindIdentExpr {
			sels = append(sels, s)
		}
		return true
	})
	if len(sels) != 2 {
		t.Fatalf("G16: view Kind has %d qualified names, want 2", len(sels))
	}
	for _, s := range sels {
		q, _ := s.X.(*syntax.IdentExpr)
		enum, member := v.prog.Info.Uses[q], v.prog.Info.NameUses[s.Name]
		if enum == nil || enum.Kind() != check.ObjTypeName || enum.Pkg() != "studio" {
			t.Fatalf("G16: %v is %v, want the studio's enum", s.X, enum)
		}
		if member == nil || member.Kind() != check.ObjMember || v.prog.Info.Types[s] != enum.Type() {
			t.Errorf("G16: %s.%s is %v typed %v, want a member of %v", enum.Name(), s.Name.Name, member, v.prog.Info.Types[s], enum.Type())
		}
	}
}

// VIEWMODEL.md G23, G16: `@menu(items, icon: gem)` names the studio's Menu and Icon members without an import.
func TestMenuAnnotationNames(t *testing.T) {
	v := loadViews(t)
	var names []*syntax.Ident
	for _, f := range v.files {
		syntax.Inspect(f, func(n syntax.Node) bool {
			a, ok := n.(*syntax.Annotation)
			if !ok || a.Name.Name != syntax.AnnMenu {
				return true
			}
			for _, arg := range a.Args {
				if q, isName := arg.Value.(*syntax.QualifiedName); isName {
					names = append(names, q.Parts[0])
				}
			}
			return true
		})
	}
	if len(names) != 2 {
		t.Fatalf("G23: %d @menu names, want 2", len(names))
	}
	for _, id := range names {
		if o := v.prog.Info.NameUses[id]; o == nil || o.Kind() != check.ObjMember || o.Pkg() != "studio" {
			t.Errorf("G23: @menu's %s names %v, want a studio member", id.Name, o)
		}
	}
}
