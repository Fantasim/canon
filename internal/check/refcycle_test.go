package check_test

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TYPES.md §10.2, §1, EVALUATION.md §1: a build reports only E3008 (or E2103) and evaluates nothing broken.
func TestRefInferenceCycleIsNeverEvaluated(t *testing.T) {
	for _, tc := range []struct {
		glob string
		code diag.Code
	}{
		{"testdata/findings/E3008_[6-9].txtar", diag.E3008.Def().Code},
		{"testdata/findings/E3008_1[0-57].txtar", diag.E3008.Def().Code},
		{"testdata/findings/E2103_4.txtar", diag.E2103.Def().Code},
	} {
		cases, err := golden.Load(tc.glob, golden.Expected(findingsFile))
		if err != nil || len(cases) == 0 {
			t.Fatalf("%s: %d cases, %v", tc.glob, len(cases), err)
		}
		for _, c := range cases {
			t.Run(filepath.Base(c.Path), func(t *testing.T) {
				res := buildOne(t, string(c.Archive.Files[0].Data))
				codes := errorCodes(res.List)
				for _, code := range codes {
					if code != tc.code {
						t.Errorf("code %s, want only %s", code, tc.code)
					}
				}
				if len(codes) == 0 {
					t.Error("no finding")
				}
			})
		}
	}
}

// TYPES.md §6.4, §5.1: `[T] + [ref T]` and parenthesized operands build with their checks holding.
func TestAcceptedAreEvaluated(t *testing.T) {
	for _, name := range []string{"refjoin", "parenoperand"} {
		t.Run(name, func(t *testing.T) {
			cases, err := golden.Load("testdata/accept/" + name + ".txtar")
			if err != nil || len(cases) != 1 {
				t.Fatalf("%d cases, %v", len(cases), err)
			}
			res := buildOne(t, string(cases[0].Archive.Files[0].Data))
			if len(res.List) != 0 {
				t.Errorf("findings %v, want none", codesOf(res.List))
			}
		})
	}
}
