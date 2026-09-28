package cppgen_test

import (
	"regexp"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// CODEGEN.md §3.5, §7.7: LoadInputs' parameter and locals (error, problems, raw, val, kPattern) hide namespace names, but its body writes every user name qualified (::kscope::…), so types named like them compile: no E8005 there.
func TestLoadInputsLocalsMeetNoUserName(t *testing.T) {
	t.Parallel()
	val, problems, kPattern := enumOf("val", "a", "b"), enumOf("problems", "x"), enumOf("kPattern", "y")
	rec := &ir.Record{Pkg: "kscope", Name: "raw", Fields: []*ir.Field{
		input("v", "V", ir.TypeRef{Kind: types.Enum, Named: val}, true, nil),
		input("p", "P", ir.TypeRef{Kind: types.Enum, Named: problems}, true, nil),
		input("k", "K", ir.TypeRef{Kind: types.Enum, Named: kPattern}, true, nil),
		input("s", "S", tString, true, bound(1, 4)),
		input("n", "N", tInt32, true, nil),
	}}
	rec.Fields[3].Pattern = regexp.MustCompile(`^[a-z]+$`)
	errorClass := &ir.Record{Pkg: "kscope", Name: "error", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	decls := []ir.Type{val, problems, kPattern, errorClass, rec}
	for _, e := range []*ir.Enum{val, problems, kPattern} {
		e.Pkg = "kscope"
	}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "kscope/out", Mode: ir.ModeData, Namespace: "kscope"}
	dir := t.TempDir()
	writeFiles(t, dir, generate(t, &ir.Package{Name: "kscope", Dir: "kscope", Types: decls, Emits: []*ir.Emit{emit}}))
	compileOnly(t, dir, []string{"kscope.gen.cpp"})
}
