package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	textFn     = "/// The notes.\n@text(\"a.txt\")\nexport fn notes() -> String { return \"hi\" }\n"
	textEmitGo = "\nemit go { out: \"@features/a\", package: \"a\", mode: %s }\n"
	plainFn    = "\n/// Doubles a flag.\nexport fn weight(heavy: Bool) -> Int { return if heavy { 2 } else { 1 } }\n"
	textFile   = "hi"
	noErrors   = "0 errors"
)

// textWorld builds package a (source) with a notes fn answering "hi".
func textWorld(t *testing.T, src string) (*world, []*ir.Package) {
	t.Helper()
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(src))
	w.check(t)
	w.calls = func(fn check.Object, _ value.Value, _ []value.Value) (value.Value, bool) {
		return &value.Str{V: textFile, T: types.StringType}, fn.Name() == "notes"
	}
	return w, w.build(t)
}

// CODEGEN.md §2.9, DECISIONS 300: a `@text` fn is a file, not API; a data or types emit of its package leaves it out (no E8013, no E8014), and its file is still written.
func TestTextFnIsNotApiInDataAndTypesEmit(t *testing.T) {
	for _, mode := range []string{"data", "types"} {
		src := "package a\n\n" + textFn + strings.Replace(textEmitGo, "%s", mode, 1)
		w, pkgs := textWorld(t, src)
		if out := w.findings(t); !strings.Contains(out, noErrors) {
			t.Errorf("mode %s: want no finding:\n%s", mode, out)
		}
		files, err := ir.TextFiles(w.prog.Packages[0], pkgs[0])
		if err != nil || len(files) != 1 || string(files[0].Content) != textFile {
			t.Errorf("mode %s: TextFiles = %v, %v", mode, files, err)
		}
	}
}

// CODEGEN.md §5.10, §5.13: a real package-level export fn beside a `@text` fn stays E8013 in data mode and E8014 in types mode.
func TestPlainFnBesideTextFnStillRefused(t *testing.T) {
	for _, c := range []struct {
		mode string
		code diag.Code
	}{{"data", diag.E8013.Def().Code}, {"types", diag.E8014.Def().Code}} {
		mode, code := c.mode, c.code
		src := "package a\n\n" + textFn + plainFn + strings.Replace(textEmitGo, "%s", mode, 1)
		w, _ := textWorld(t, src)
		if out := w.findings(t); !strings.Contains(out, "["+string(code)+"]") || strings.Contains(out, "notes") {
			t.Errorf("mode %s: want %s on weight only:\n%s", mode, code, out)
		}
	}
}
