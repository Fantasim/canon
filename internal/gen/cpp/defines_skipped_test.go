package cppgen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	skippedDocMacro  = "SK_DOC"
	skippedStrMacro  = "SK_STR"
	skippedCodeMacro = "SK_CODE"
)

// skippedPackage is skip.user, a types-mode package that reads a package writing SK_DOC, SK_STR and SK_CODE as defines: SK_DOC only in a doc comment, SK_STR only in a string default, SK_CODE as a class name.
func skippedPackage() *ir.Package {
	str := field("label", "label", "Set "+skippedDocMacro+" before use.", tString)
	str.Default = &value.Str{V: skippedStrMacro}
	holder := &ir.Record{Pkg: "skip.user", Name: "Holder", Fields: []*ir.Field{str}}
	code := &ir.Record{Pkg: "skip.user", Name: skippedCodeMacro, Fields: []*ir.Field{field("n", "n", "", tInt)}}
	emit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "skip/user/out", Mode: ir.ModeTypes, Namespace: "skip::user"}
	return &ir.Package{
		Name: "skip.user", Dir: "skip/user", Types: []ir.Type{holder, code}, Emits: []*ir.Emit{emit},
		Imports: []*ir.PackageRef{{Name: "skip.defs", Dir: "skip/defs", CppDefines: []string{skippedCodeMacro, skippedDocMacro, skippedStrMacro}}},
	}
}

// CODEGEN.md §7.9, DECISIONS 404: a macro met only in a doc comment or a string constant is not wrapped, one met in code is.
func TestDefinesSkipCommentsAndStrings(t *testing.T) {
	t.Parallel()
	wrapsCode := false
	for _, f := range generate(t, skippedPackage()) {
		text := string(f.Content)
		for _, m := range []string{skippedDocMacro, skippedStrMacro} {
			if strings.Contains(text, `push_macro("`+m+`")`) {
				t.Errorf("%s wraps %s, which only a comment or a string names", f.Path, m)
			}
		}
		wrapsCode = wrapsCode || strings.Contains(text, `push_macro("`+skippedCodeMacro+`")`)
	}
	if !wrapsCode {
		t.Errorf("no file wraps %s, which a class name spells", skippedCodeMacro)
	}
}
