package gogen_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// foreignSizes is ui, a go emit in uiMode with a translated sizeFor(n Int) -> ref core.sizes; core's go emit is in coreMode.
func foreignSizes(uiMode, coreMode ir.Mode) *ir.Package {
	size := &ir.Record{Pkg: "core", Name: "Size", Fields: []*ir.Field{wired("cap", "cap", "", intT)}}
	refT := ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "core", Value: "sizes", Elem: size}}
	ref := func(id string) *ir.Lit { return lit(refT, &value.Ref{Key: value.Key{S: id}}) }
	cond := bin(ir.OpGt, boolT, bin(ir.OpAdd, intT, prm(0, intT), lit(intT, num(1))), lit(intT, num(5)))
	fn := spec{
		name: "sizeFor", params: params(intT, "n"), result: refT,
		body: &ir.If{T: refT, Cond: cond, Then: ref("large"), Else: ref("small")},
		vecs: []vec{
			ok(nil, &value.Ref{Key: value.Key{S: "large"}}, num(7)),
			fails(nil, eOverflow, num(math.MaxInt64)),
		},
	}.build(1)
	ui := goData("demo/ui", "ui")
	ui.Mode = uiMode
	core := goData("demo/core", "core")
	core.Mode = coreMode
	return &ir.Package{
		Name: "ui", Dir: "demo/ui", Fns: []*ir.ExportFn{fn}, Emits: []*ir.Emit{ui},
		Imports: []*ir.PackageRef{{Name: "core", Dir: "demo/core", Emits: []*ir.Emit{core}}},
	}
}

// generatedText is every generated file of p's first emit, joined.
func generatedText(t *testing.T, p *ir.Package) string {
	t.Helper()
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var all strings.Builder
	for _, f := range files {
		if !strings.Contains(string(f.Content), "\npackage rt\n") {
			all.Write(f.Content)
		}
	}
	return all.String()
}

// CODEGEN.md §5.3, §5.8: a table ref's literal, zero and verb follow the ref's own package, not the importer's mode.
func TestPureRefsFollowTheOwnersMode(t *testing.T) {
	t.Run("owner baked, importer data", func(t *testing.T) {
		src := generatedText(t, foreignSizes(ir.ModeData, ir.ModeBaked))
		if !strings.Contains(src, "core.SizeIDLarge") {
			t.Errorf("no core.SizeIDLarge:\n%s", src)
		}
		if strings.Contains(src, `core.SizeID("`) {
			t.Errorf("a baked id converted from a string:\n%s", src)
		}
	})
	t.Run("owner data, importer baked", func(t *testing.T) {
		src := generatedText(t, foreignSizes(ir.ModeBaked, ir.ModeData))
		if !strings.Contains(src, `core.SizeID("large")`) {
			t.Errorf(`no core.SizeID("large"):`+"\n%s", src)
		}
		if strings.Contains(src, "core.SizeIDLarge") {
			t.Errorf("a data id used as a constant:\n%s", src)
		}
		if !strings.Contains(src, fmt.Sprintf(`{math.MaxInt64, "", %q}`, eOverflow)) {
			t.Errorf("a data id's failing row does not want the empty string:\n%s", src)
		}
	})
}
