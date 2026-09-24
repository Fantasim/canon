package ir_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
)

// TestDefaultGoPackage is DECISIONS 213 and 215 (CODEGEN.md §2.1): a go emit that writes no package takes the last element of out as declared, none for the project directory itself (never "."), resolving an unrooted out from the directory of the file holding the emit (WIRE.md §2.2), exactly as check validates it; a dependency's package check refused (E8009), written or defaulted, is not refused again at its importer (no E8011 there).
func TestDefaultGoPackage(t *testing.T) {
	e8009, e8007 := string(diag.E8009.Def().Code), string(diag.E8007.Def().Code)
	cases := []struct {
		name           string
		files          []string
		pkg, goPackage string
		dir            string
		codes          []string
	}{
		{"unrooted out from the file holding the emit", []string{"features/k/k.canon", `package features.k

/// A colour.
enum Tone { red, blue }
`, "features/k/sub/s.canon", `package features.k

emit go { out: "." }
`}, "features.k", "sub", "features/k/sub", nil},
		{"project directory names no package", []string{"b/b.canon", `package b

/// A colour.
enum Tone { red, blue }

emit go { out: ".." }
`, "a/a.canon", `package a

import b { Tone }

/// A badge.
record Badge {
  /// Its colour.
  tone: Tone
}

emit go { out: "@features/a", package: "a" }
`}, "b", "", ".", []string{e8009, e8007}},
		{"a written and a defaulted package refused", []string{"b/b.canon", `package b

/// A colour.
enum Tone { red, blue }

emit go { out: "@features/b", package: "1b" }
`, "c/c.canon", `package c

/// A size.
enum Size { small, large }

emit go { out: "@features/c/1b" }
`, "a/a.canon", `package a

import b { Tone }
import c { Size }

/// A badge.
record Badge {
  /// Its colour.
  tone: Tone
  /// Its size.
  size: Size
}

emit go { out: "@features/a", package: "a" }
`}, "c", "1b", "features/c/1b", []string{e8009, e8009}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			for i := 0; i+1 < len(tc.files); i += 2 {
				w.add(t, tc.files[i], []byte(tc.files[i+1]))
			}
			e := goEmit(w.build(t), tc.pkg)
			if e == nil || e.GoPackage != tc.goPackage || e.Dir != tc.dir {
				t.Fatalf("go emit of %s = %+v, want package %q in %q", tc.pkg, e, tc.goPackage, tc.dir)
			}
			out := w.findings(t)
			var codes []string
			for _, m := range reFindingCode.FindAllStringSubmatch(out, -1) {
				codes = append(codes, m[1])
			}
			if !slices.Equal(codes, tc.codes) {
				t.Errorf("codes %v, want %v:\n%s", codes, tc.codes, out)
			}
		})
	}
}

// goEmit is the go emit of package name among pkgs, or nil.
func goEmit(pkgs []*ir.Package, name string) *ir.Emit {
	for _, p := range pkgs {
		for _, e := range p.Emits {
			if p.Name == name && e.Target == ir.TargetGo {
				return e
			}
		}
	}
	return nil
}
