package progen_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// climbOperator moves a cpp or ts emit under @source, its import staying in the project (CODEGEN.md §2.8).
func climbOperator() operator {
	return op(diag.E8025.Def().Code, "CODEGEN.md §2.8 (relative path above the project)", func(tg target) []progen.Site {
		return append(climbingOut(tg, "cpp", `"@source/Generated/zzclimb/"`), climbingOut(tg, "ts", `"@source/Generated/zzclimb.ts"`)...)
	})
}

// climbingOut moves the single out of tg's kind emit to value, for each package it imports whose
// kind emit writes inside the project.
func climbingOut(tg target, kind, value string) []progen.Site {
	var out []progen.Site
	for _, d := range emitsOf(tg, kind) {
		f := option(d, "out")
		if f == nil || !strings.HasPrefix(text(tg, f.Value), `"`) {
			continue
		}
		for _, imp := range nodes[*syntax.Import](tg) {
			if !insideProject(*tg.all, text(tg, imp.Path), kind) {
				continue
			}
			s, e := span(tg, f.Value)
			st := site(replace(s, e, value))
			st.Packages = []string{tg.pkg}
			out = append(out, st)
		}
	}
	return out
}

// insideProject reports a package of files with a kind emit whose single out is unrooted or under
// a root inside the project.
func insideProject(files []target, pkg, kind string) bool {
	for _, o := range files {
		if o.pkg != pkg {
			continue
		}
		for _, d := range emitsOf(o, kind) {
			f := option(d, "out")
			if f == nil || !strings.HasPrefix(text(o, f.Value), `"`) {
				continue
			}
			out := optionText(o, d, "out")
			if !strings.HasPrefix(out, "@") || strings.HasPrefix(out, "@features/") || strings.HasPrefix(out, "@pipeline_go/") {
				return true
			}
		}
	}
	return false
}
