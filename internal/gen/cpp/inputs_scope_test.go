package cppgen_test

import (
	"regexp"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// CODEGEN.md §3.5, §7.7: LoadInputs' parameter and locals (error, problems, raw, val, kPattern, and kPattern2 for a chain's second pattern) hide namespace names, but its body writes every user name qualified (::kscope::…), so types named like them compile: no E8005 there.
func TestLoadInputsLocalsMeetNoUserName(t *testing.T) {
	t.Parallel()
	val, problems, kPattern, kPattern2 := enumOf("val", "a", "b"), enumOf("problems", "x"), enumOf("kPattern", "y"), enumOf("kPattern2", "z")
	rec := &ir.Record{Pkg: "kscope", Name: "raw", Fields: []*ir.Field{
		input("v", "V", ir.TypeRef{Kind: types.Enum, Named: val}, true, nil),
		input("p", "P", ir.TypeRef{Kind: types.Enum, Named: problems}, true, nil),
		input("k", "K", ir.TypeRef{Kind: types.Enum, Named: kPattern}, true, nil),
		input("s", "S", tString, true, bound(1, 4)),
		input("n", "N", tInt32, true, nil),
		input("k2", "K2", ir.TypeRef{Kind: types.Enum, Named: kPattern2}, true, nil),
	}}
	rec.Fields[3].Patterns = []*regexp.Regexp{regexp.MustCompile(`^[a-z]+$`), regexp.MustCompile(`^.{1,3}$`)}
	errorClass := &ir.Record{Pkg: "kscope", Name: "error", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	decls := []ir.Type{val, problems, kPattern, kPattern2, errorClass, rec}
	for _, e := range []*ir.Enum{val, problems, kPattern, kPattern2} {
		e.Pkg = "kscope"
	}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "kscope/out", Mode: ir.ModeData, Namespace: "kscope"}
	dir := t.TempDir()
	writeFiles(t, dir, generate(t, &ir.Package{Name: "kscope", Dir: "kscope", Types: decls, Emits: []*ir.Emit{emit}}))
	compileOnly(t, dir, []string{"kscope.gen.cpp"})
}
