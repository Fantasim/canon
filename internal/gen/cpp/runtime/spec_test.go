package cppruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/specfence"
)

const (
	codegenSpec   = "../../../../spec/CODEGEN.md"
	specGlob      = "../../../../spec/*.md"
	headerHeading = "### 7.4"
	fenceLang     = "cpp"
	scopeSep      = "::"
)

// canonName is a `canon::<Name>` of the specification, every `::` segment included; a name followed by `*` names a family, not one name.
var canonName = regexp.MustCompile(`\bcanon((?:::[A-Za-z_][A-Za-z0-9_]*)+)(\*?)`)

// CODEGEN.md §7.4 (DECISIONS 415, 436): canon_runtime.h is normative text; the embedded copy, @CANON_STAMP@ where the stamp goes, is the spec's, byte for byte.
func TestHeaderIsTheSpecs(t *testing.T) {
	t.Parallel()
	if want := specfence.Block(t, codegenSpec, headerHeading, fenceLang); headerText != want {
		t.Errorf("canon_runtime.h.txt differs from CODEGEN.md §7.4 (%d bytes, the spec's %d)", len(headerText), len(want))
	}
}

// staleNames are the `file:line: canon::Name` of a spec document with a segment that is no identifier in ids; a family (`canon::Name*`) is skipped.
func staleNames(text, file string, ids []string) []string {
	var out []string
	for n, line := range strings.Split(text, "\n") {
		for _, m := range canonName.FindAllStringSubmatch(line, -1) {
			if m[2] == "" && !allIn(ids, strings.Split(m[1], scopeSep)[1:]) {
				out = append(out, fmt.Sprintf("%s:%d: canon%s", file, n+1, m[1]))
			}
		}
	}
	return out
}

// allIn reports whether every name is in ids.
func allIn(ids, names []string) bool {
	for _, n := range names {
		if !slices.Contains(ids, n) {
			return false
		}
	}
	return true
}

// CODEGEN.md §7.4 (DECISIONS 415): a name followed by `*` names a family and is skipped, a name any of whose segments the headers lack is reported with its line.
func TestStaleNames(t *testing.T) {
	t.Parallel()
	text := "a canon::json::Decoder\nb `canon::Gone` canon::Fam* x\nc canon::json::Decodr"
	got := staleNames(text, "doc.md", []string{"json", "Decoder"})
	if want := []string{"doc.md:2: canon::Gone", "doc.md:3: canon::json::Decodr"}; !slices.Equal(got, want) {
		t.Errorf("%v, want %v", got, want)
	}
}

// CODEGEN.md §7.4, §7.5 (DECISIONS 415): every `canon::<Name>` the specification names is an identifier of the embedded runtime headers.
func TestSpecNamesTheRuntime(t *testing.T) {
	t.Parallel()
	docs, err := filepath.Glob(specGlob)
	if err != nil || len(docs) == 0 {
		t.Fatalf("no specification found at %s: %v", specGlob, err)
	}
	ids := Identifiers()
	for _, doc := range docs {
		data, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		for _, stale := range staleNames(string(data), filepath.Base(doc), ids) {
			t.Errorf("%s is not an identifier of the runtime headers", stale)
		}
	}
}
