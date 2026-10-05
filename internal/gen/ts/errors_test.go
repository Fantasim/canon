package tsgen_test

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/diag"
	tsgen "github.com/fantasim/canonlang/internal/gen/ts"
	"github.com/fantasim/canonlang/internal/ir"
)

const (
	multiline       = "(?m)"
	collisionPrefix = "/// A.\npackage a\n\n"
	collisionSuffix = "\nemit ts { out: \"@features/a/a.ts\" }\n"
)

// CODEGEN.md §3.5, §8.2, DECISIONS 278, 323: two generated names equal in the module are E8005 from stage E, so check reports them and the generator never meets one: a value named like a helper, like an enum's table, two cases' types, a type named like the CanonRow helper.
func TestCollisionsAreE8005(t *testing.T) {
	cases := []struct{ name, body string }{
		{"helper", "/// V.\nlet canonFreeze: Int = 1\n"},
		{"enum table", "/// E.\nenum Tone { red }\n\n/// V.\nlet ToneMembers: Int = 1\n"},
		{"case type", "/// V.\nvariant V {\n  /// C.\n  c_d { /// X\n x: Int }\n}\n\n/// W.\nrecord VCD {\n  /// X.\n  x: Int\n}\n"},
		{"row helper", "/// R.\nrecord CanonRow {\n  /// X.\n  x: Int\n}\n"},
	}
	for _, c := range cases {
		w := buildWorld(t, []txtar.File{{Name: "a/a.canon", Data: []byte(collisionPrefix + c.body + collisionSuffix)}})
		all, _ := w.findings(t)
		if !slices.ContainsFunc(all, func(f diag.Finding) bool { return f.Code == diag.E8005.Def().Code }) {
			t.Errorf("%s: %v", c.name, all)
		}
	}
}

// The helper units the module writes are the names stage E reserves (ir.TSHelperNames): CODEGEN.md §8.2's block, CanonRow included (DECISIONS 323), and the decoders'.
func TestHelperNamesAreStageEs(t *testing.T) {
	decl := regexp.MustCompile(multiline + tsgen.DeclPattern)
	var got []string
	for _, f := range []string{helperFile, "runtime/decode.ts.txt"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range decl.FindAllStringSubmatch(string(data), -1) {
			got = append(got, m[1])
		}
	}
	want := ir.TSHelperNames()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("units %v, reserved %v", got, want)
	}
}

// CODEGEN.md §2.1: a generator is called on a ts emit with a file; anything else is refused with a sentinel a caller compares.
func TestGenerateRefusals(t *testing.T) {
	p := &ir.Package{Name: "a", Dir: "a"}
	cases := []struct {
		name string
		p    *ir.Package
		e    *ir.Emit
		want error
	}{
		{"nil package", nil, &ir.Emit{Target: ir.TargetTS}, tsgen.ErrTarget},
		{"nil emit", p, nil, tsgen.ErrTarget},
		{"go emit", p, &ir.Emit{Target: ir.TargetGo, FileName: "a.ts"}, tsgen.ErrTarget},
		{"no file name", p, &ir.Emit{Target: ir.TargetTS}, tsgen.ErrMalformed},
		{"no dir", &ir.Package{Name: "a"}, &ir.Emit{Target: ir.TargetTS, FileName: "a.ts"}, tsgen.ErrMalformed},
	}
	for _, c := range cases {
		if _, err := tsgen.Generate(c.p, c.e); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}
