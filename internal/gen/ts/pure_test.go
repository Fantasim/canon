package tsgen_test

import (
	"regexp"
	"strings"
	"testing"

	tsgen "github.com/fantasim/canonlang/internal/gen/ts"
)

const (
	itemSep      = "\n\n"
	constItem    = "const "
	exportConst  = "export const "
	docLead      = "/**"
	bannerLead   = "// ----"
	bannersInRun = 3
	snippetLen   = 40
)

// constructionCall is a call or `new` the generator writes to build a table, a frozen value or an enum table at the top level of a module.
var constructionCall = regexp.MustCompile(`Object\.freeze\(|canonFreeze[<(]|canonList<|canonTable\(|canonIndex\(|new (?:CanonMap|Map)<`)

// withoutHelperBlock is the text without the lines from the first banner to the third (CODEGEN.md §8.2).
func withoutHelperBlock(src string) string {
	var kept []string
	banners := 0
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, bannerLead) {
			banners++
			continue
		}
		if banners == 0 || banners >= bannersInRun {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// valueItems are the blank-line-separated items of src that declare a module-level const, doc comments aside.
func valueItems(src string) []string {
	var out []string
	for _, item := range strings.Split(withoutHelperBlock(src), itemSep) {
		body := item
		for strings.HasPrefix(body, docLead) {
			end := strings.Index(body, "*/\n")
			if end < 0 {
				break
			}
			body = body[end+len("*/\n"):]
		}
		if strings.HasPrefix(body, constItem) || strings.HasPrefix(body, exportConst) {
			out = append(out, item)
		}
	}
	return out
}

// constructionsOf are the constructions of the module-level consts of src and, of them, the ones not marked.
func constructionsOf(src string) (seen int, unmarked []string) {
	for _, item := range valueItems(src) {
		for _, at := range constructionCall.FindAllStringIndex(item, -1) {
			seen++
			if !strings.HasSuffix(item[:at[0]], tsgen.PureCall) {
				unmarked = append(unmarked, item[at[0]:min(at[0]+snippetLen, len(item))])
			}
		}
	}
	return seen, unmarked
}

// CODEGEN.md §8.1 "Bundlers", §2.5 T22, DECISIONS 433: every call or `new` that builds a top-level table, frozen value or enum table is marked /*#__PURE__*/, however deep in the expression, so a bundler drops the declaration when nothing reads it.
func TestTopLevelConstructionsArePure(t *testing.T) {
	total := 0
	for _, f := range append(generatedFiles(t), exampleFiles(t)...) {
		if strings.HasSuffix(f.Path, ".conformance.test.ts") {
			continue
		}
		seen, unmarked := constructionsOf(string(f.Content))
		total += seen
		for _, u := range unmarked {
			t.Errorf("%s: %q is not marked %s", f.Path, u, strings.TrimSpace(tsgen.PureCall))
		}
	}
	if total == 0 {
		t.Fatal("no construction found: the test looks at nothing")
	}
}
