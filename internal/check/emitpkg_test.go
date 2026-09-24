package check_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// WIRE.md §2.2: an unrooted out starts at the directory of the file holding the emit.
func TestDefaultGoPackageFromFileDir(t *testing.T) {
	for _, tc := range []struct {
		emit string
		want []diag.Code
	}{
		{emit: `emit go { out: ".." }`},
		{emit: `emit go { out: "." }`},
		{emit: `emit go { out: "../.." }`, want: []diag.Code{diag.E8009.Def().Code}},
	} {
		t.Run(tc.emit, func(t *testing.T) {
			set := &source.FileSet{}
			src, err := set.Add("a/b/c.canon", "/a/b/c.canon", []byte("package a\n\n"+tc.emit+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			bags := check.Bags{builtPackage: diag.NewBag(set, builtPackage)}
			f := syntax.Parse(src, syntax.FileSource, bags[builtPackage])
			check.Check(context.Background(), exampleProject(), []*syntax.File{f}, bags, literalFolder{})
			if got := errorCodes(bags[builtPackage].Findings()); !slices.Equal(got, tc.want) {
				t.Errorf("codes %v, want %v", got, tc.want)
			}
		})
	}
}

// CODEGEN.md §2.1, DECISIONS 213: the default package, the last element of out, is validated when check can place it.
func TestDefaultGoPackage(t *testing.T) {
	e8009 := []diag.Code{diag.E8009.Def().Code}
	for _, tc := range []struct {
		emit string
		want []diag.Code
	}{
		{emit: `emit go { out: "@features/1b" }`, want: e8009},
		{emit: `emit go { out: "@features/gen/func" }`, want: e8009},
		{emit: `emit go { out: "@features/gen/type/" }`, want: e8009},
		{emit: `emit go { out: "gen/my-pkg" }`, want: e8009},
		{emit: `emit go { out: "@features/x/../select" }`, want: e8009},
		{emit: `emit go { out: "@features/gen/items" }`},
		{emit: `emit go { out: "@features" }`},
		{emit: `emit go { out: "@features/x/.." }`},
		{emit: `emit go { out: "." }`},
		{emit: `emit go { out: "gen" }`},
		{emit: `emit go { out: ".." }`, want: e8009},
		{emit: `emit go { out: "../", package: "top" }`},
		{emit: `emit go { out: "C:/gen/1b" }`},
		{emit: `emit go { out: "gen\\1b" }`},
		{emit: `emit go { out: "gen//1b" }`},
		{emit: `emit go { out: "@features/1b", package: "items" }`},
		{emit: `emit go { out: "@nowhere/1b" }`},
		{emit: `emit go { out: "@features/../1b" }`},
		{emit: `emit go { out: "../../1b" }`},
		{emit: `emit go { out: "" }`},
		{emit: `emit cpp { out: "@features/1b" }`},
	} {
		t.Run(tc.emit, func(t *testing.T) {
			b := checkBuilt(t, "package "+builtPackage+"\n\n"+tc.emit+"\n")
			if want := sortedCodes(tc.want); !slices.Equal(b.codes, want) {
				t.Errorf("codes %v, want %v:\n%s", b.codes, want, b.out)
			}
		})
	}
}
