package cppgen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// unionPackages are demo.kinds, an enum of smallN members, then two types-mode importers whose
// record holds a union "Title" | Kind: demo.tagged's header includes kinds' (another field holds a
// Kind), demo.bare's does not, so only the first checks through KindFromWire (DECISIONS 392).
func unionPackages() (kinds, tagged, bare *ir.Package) {
	kind := &ir.Enum{Pkg: "demo.kinds", Name: "Kind"}
	for i := range smallN {
		w := fmt.Sprintf("k%d", i)
		kind.Members = append(kind.Members, &ir.EnumMember{Name: w, Wire: w, Index: i})
	}
	kinds = &ir.Package{Name: "demo.kinds", Dir: "demo/kinds", Types: []ir.Type{kind},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "demo/kinds/out", Mode: ir.ModeTypes, Namespace: "demo::kinds"}}}
	tKind := ir.TypeRef{Kind: types.Enum, Named: kind}
	union := ir.TypeRef{Kind: types.LitUnion, Elem: &tKind, Literals: []string{"Title"}}
	importer := func(pkg string, fields ...*ir.Field) *ir.Package {
		rec := &ir.Record{Pkg: pkg, Name: "Pick", Fields: fields}
		last := pkg[strings.LastIndex(pkg, ".")+1:]
		return &ir.Package{Name: pkg, Dir: "demo/" + last, Types: []ir.Type{rec},
			Imports: []*ir.PackageRef{{Name: kinds.Name, Dir: kinds.Dir, Emits: kinds.Emits}},
			Emits:   []*ir.Emit{{Target: ir.TargetCpp, Dir: "demo/" + last + "/out", Mode: ir.ModeTypes, Namespace: "demo::" + last}}}
	}
	tagged = importer("demo.tagged", field("kind", "kind", "", union), field("other", "other", "", tKind))
	bare = importer("demo.bare", field("kind", "kind", "", union))
	return kinds, tagged, bare
}

// unionWant is what testdata/main/union_import_main.cpp prints.
const unionWant = "tagged k7: ok\ntagged Title: ok\ntagged k9: Pick: $.kind: unknown value k9\n" +
	"bare k7: ok\nbare Title: ok\nbare k9: Pick: $.kind: unknown value k9\n"

// CODEGEN.md §5.13, DECISIONS 392: a union over another package's 8-member enum checks through KindFromWire where the header includes that package, by its wires where it does not, and both refuse the same texts.
func TestUnionOverImportedEnum(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	kinds, tagged, bare := unionPackages()
	sources := []string{"main.cpp"}
	for _, p := range []*ir.Package{kinds, tagged, bare} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		files := generate(t, p)
		writeFiles(t, out, files)
		last := p.Name[strings.LastIndex(p.Name, ".")+1:]
		sources = append(sources, cppEmit(p).Dir+"/"+last+".gen.cpp")
		for _, f := range files {
			search := strings.Contains(string(f.Content), "KindFromWire(out.kind_)")
			if strings.HasSuffix(f.Path, ".gen.cpp") && p != kinds && search != (p == tagged) {
				t.Errorf("%s: the check searches KindFromWire: %v, want %v", p.Name, search, p == tagged)
			}
		}
	}
	copyFile(t, filepath.Join("testdata", "main", "union_import_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	for _, out := range buildAndRun(t, dir, sources) {
		if out != unionWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, unionWant)
		}
	}
}
