package gogen_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// chainPkg is geninputschain: Config.code is an input whose alias chain carries two patterns,
// innermost first (`type Code = String(/^[a-z]+$/)`, `type Short = Code(/^.{1,4}$/)`).
func chainPkg() *ir.Package {
	code := &ir.Field{
		Name: "code", Type: strT, Optional: true, Input: &types.Input{Env: "GENINPUTSCHAIN_CODE"},
		Patterns: []*regexp.Regexp{regexp.MustCompile(`^[a-z]+$`), regexp.MustCompile(`^.{1,4}$`)},
	}
	rec := &ir.Record{Pkg: "geninputschain", Name: "Config", Fields: []*ir.Field{code}}
	return &ir.Package{
		Name: "geninputschain", Dir: "geninputschain", Types: []ir.Type{rec},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: "geninputschain/out/go",
			GoImport: dataModule + "/geninputschain/out/go", Mode: ir.ModeBaked, GoPackage: "geninputschain",
		}},
	}
}

// TestInputsPatternChain is TYPES.md §7.4 ("both are checked"), EVALUATION.md §11.3 step 3 and CODEGEN.md §7.7: one compiled pattern per pattern of the chain, input_<T>_<store>_Pattern then _Pattern2, checked innermost first; the compiled loader refuses a text that passes the outer pattern but fails the inner one, and the reverse.
func TestInputsPatternChain(t *testing.T) {
	files := generateData(t, chainPkg())
	src := string(files["geninputschain/out/go/geninputschain.gen.go"])
	for _, want := range []string{
		`input_Config_code_Pattern  = regexp.MustCompile("^[a-z]+$")`,
		`input_Config_code_Pattern2 = regexp.MustCompile("^.{1,4}$")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("want %s in generated source:\n%s", want, src)
		}
	}
	inner, outer := strings.Index(src, "case !input_Config_code_Pattern.MatchString("), strings.Index(src, "case !input_Config_code_Pattern2.MatchString(")
	if inner < 0 || outer < inner {
		t.Errorf("want the inner pattern checked before the outer one:\n%s", src)
	}
	runData(t, files, "geninputschain/out/go", "testdata/smoke/inputs_chain_test.go", nil)
}
