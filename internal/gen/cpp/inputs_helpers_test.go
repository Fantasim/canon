package cppgen_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
	"github.com/fantasim/canonlang/internal/types"
)

// helperDef is a §7.7 helper's definition line in a .gen.cpp, after the anonymous namespace opens.
var helperDef = regexp.MustCompile(`(?m)^bool (\w+)\(`)

const anonymous = "\nnamespace {\n"

// oneInput is a package whose only input field has type t.
func oneInput(pkg string, t ir.TypeRef, extra ...ir.Type) *ir.Package {
	rec := &ir.Record{Pkg: pkg, Name: "Config", Fields: []*ir.Field{input("v", "V", t, true, nil)}}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: pkg + "/out", Mode: ir.ModeData, Namespace: pkg}
	return &ir.Package{Name: pkg, Dir: pkg, Types: append(extra, rec), Emits: []*ir.Emit{emit}}
}

// CODEGEN.md §7.7, §9: only the helpers the inputs use, in ir's order, so -Wall -Werror passes; an enum, @json(codes) or not, needs EnvText only.
func TestInputHelpersPerKind(t *testing.T) {
	codes := ir.TypeRef{Kind: types.Int, Bits: 8}
	plain, coded := enumOf("E", "a", "b"), enumOf("E", "a", "b")
	coded.JSONCodes, coded.Codes = true, &codes
	coded.Members[0].Code, coded.Members[1].Code = 1, 2
	tests := []struct {
		pkg     string
		t       ir.TypeRef
		extra   []ir.Type
		helpers []string
	}{
		{"kbool", tBool, nil, []string{"EnvText", "ParseBoolLiteral"}},
		{"kint", tInt32, nil, []string{"EnvText", "IsDecDigit", "AllDigits", "ParseIntLiteral"}},
		{"kfloat", tFloat, nil, []string{"EnvText", "IsDecDigit", "AllDigits", "ParseFloatLiteral"}},
		{"kfloat32", tFloat32, nil, []string{"EnvText", "IsDecDigit", "AllDigits", "ParseFloatLiteral"}},
		{"kduration", tDuration, nil, []string{"EnvText", "IsDecDigit", "AllDigits", "DurationDigits", "ParseDurationLiteral"}},
		{"kstring", tString, nil, []string{"EnvText", "ParseStringLiteral"}},
		{"kenum", ir.TypeRef{Kind: types.Enum, Named: plain}, []ir.Type{plain}, []string{"EnvText"}},
		{"kcodes", ir.TypeRef{Kind: types.Enum, Named: coded}, []ir.Type{coded}, []string{"EnvText"}},
	}
	dir := t.TempDir()
	var sources []string
	for _, tt := range tests {
		for _, e := range tt.extra {
			e.(*ir.Enum).Pkg = tt.pkg
		}
		files := generate(t, oneInput(tt.pkg, tt.t, tt.extra...))
		writeFiles(t, dir, files)
		if got, want := helpersIn(files, tt.pkg+".gen.cpp"), append(slices.Clone(tt.helpers), "LoadInputs"); !slices.Equal(got, want) {
			t.Errorf("%s: helpers %v, want %v", tt.pkg, got, want)
		}
		sources = append(sources, tt.pkg+".gen.cpp")
	}
	compileOnly(t, dir, sources)
}

// helpersIn are the functions a .gen.cpp defines from its anonymous namespace on.
func helpersIn(files []ir.File, path string) []string {
	var got []string
	for _, f := range files {
		at := bytes.Index(f.Content, []byte(anonymous))
		if f.Path != path || at < 0 {
			continue
		}
		for _, m := range helperDef.FindAllSubmatch(f.Content[at:], -1) {
			got = append(got, string(m[1]))
		}
	}
	return got
}

// compileOnly compiles each source of dir with every compiler, -Werror included, and links nothing.
func compileOnly(t *testing.T, dir string, sources []string) {
	t.Helper()
	compilers, include := cxx.Toolchain(t)
	for _, cc := range compilers {
		args := append(append([]string(nil), cxx.Flags...), "-I", dir, "-I", include, "-c")
		ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
		cmd := exec.CommandContext(ctx, cc, append(args, sources...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%s: %v\n%s", filepath.Base(cc), err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, strings.TrimSuffix(sources[0], ".cpp")+".o")); err != nil {
		t.Errorf("no object file: %v", err)
	}
}
