package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const docSplicePkg = "demo.docsplice"

// docSplicePackage is demo.docsplice in types mode, whose record, field, enum and member docs have a line ending in a backslash, one holding `*/` and an empty line, next to docs that keep `///` (CODEGEN.md §2.6, CG-06, DECISIONS 423).
func docSplicePackage() *ir.Package {
	kind := &ir.Enum{Pkg: docSplicePkg, Name: "Kind", Doc: "A kind of splice \\", Members: []*ir.EnumMember{
		{Name: "a", Wire: "a", Doc: "The first member \\"},
		{Name: "b", Wire: "b", Index: 1, Doc: "The second member, plain."},
		{Name: "c", Wire: "c", Index: 2, Doc: "A comment end */ here\nand a line ending in \\\n\nafter an empty line"},
		{Name: "d", Wire: "d", Index: 3, Doc: "Plain, with a comment end */ and a path C:\\dir\\x"},
		{Name: "e", Wire: "e", Index: 4, Doc: "Reads src/*.json \\\nthen /*/ and */* and */*/ too\nand a comment /* start"},
		{Name: "f", Wire: "f", Index: 5, Doc: "Blanks follow the backslash \\ \t\nand std::vector and std::chrono::seconds are only words"},
		{Name: "g", Wire: "g", Index: 6, Doc: "A trigraph ends this line ??/\nand ??/ sits mid line, and a run ends ???/"},
		{Name: "h", Wire: "h", Index: 7, Doc: "A block with a trigraph ??/\nand a backslash \\"},
	}}
	item := &ir.Record{Pkg: docSplicePkg, Name: "Item", Doc: "A record whose doc ends in a backslash \\\nand goes on.", Fields: []*ir.Field{
		field("label", "label", "The label, ending in a backslash \\", tString),
		field("count", "count", "The count, plain.", tInt),
		field("dir", "dir", "The directory, ending in a slash: src/\\", tString),
		field("kind", "kind", "The kind, a path like C:\\dir\\ and */ closed \\", ir.TypeRef{Kind: types.Enum, Named: kind}),
	}}
	return &ir.Package{
		Name: docSplicePkg, Dir: "demo/docsplice", Types: []ir.Type{kind, item},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/docsplice/out", Mode: ir.ModeTypes, Namespace: "demo::docsplice"}},
	}
}
