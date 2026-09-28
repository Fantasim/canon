package i18n_test

import (
	"fmt"
	"strings"
	"testing"
)

// I18N.md U1: a unit's suffix may be a `const` declared in another file, with an
// interpolation; the file used to reconstruct its source text must be that const's own file,
// not the units let's (a fixed panic: syntax.(*File).Span indexed the wrong file's tokens).
func TestStudioUnitSuffixConstInAnotherFile(t *testing.T) {
	src := map[string]string{
		"studio/studio.canon": `package studio

/// Top-level sections.
enum Menu { events }

/// How numbers are shown.
record UnitSpec {
  /// Appended to the number.
  suffix: String = ""
}

let units: table UnitSpec = {
  penya { suffix: Suffix }
}
`,
		"studio/suffixes.canon": `package studio

/// A base word.
const Base = "PV"

/// The actual suffix, elsewhere.
const Suffix = " Base {Base}"
`,
	}
	cat := catalogue(t, src, "studio")
	e, ok := cat.Lookup("units.penya.suffix")
	if !ok {
		t.Fatalf("missing units.penya.suffix in %v", keys(cat))
	}
	if e.Text != " Base {Base}" {
		t.Errorf("units.penya.suffix = %q, want %q", e.Text, " Base {Base}")
	}
}

// The same, with the const far down a long file: its token position is far past the short
// units file's own token count, so using the wrong file panics with an out-of-range index.
func TestStudioUnitSuffixConstFarDownLongFile(t *testing.T) {
	var long strings.Builder
	long.WriteString("package studio\n\n")
	for i := range 200 {
		fmt.Fprintf(&long, "/// filler %d\nconst Filler%d = %d\n\n", i, i, i)
	}
	long.WriteString("/// The base.\nconst Base = \"b\"\n\n/// The actual suffix, far down a long file.\nconst Suffix = \" {Base} PV\"\n")

	src := map[string]string{
		"studio/studio.canon": `package studio

/// Top-level sections.
enum Menu { events }

/// How numbers are shown.
record UnitSpec {
  /// Appended to the number.
  suffix: String = ""
}

let units: table UnitSpec = {
  penya { suffix: Suffix }
}
`,
		"studio/long.canon": long.String(),
	}
	cat := catalogue(t, src, "studio")
	e, ok := cat.Lookup("units.penya.suffix")
	if !ok || e.Text != " {Base} PV" {
		t.Errorf("units.penya.suffix = %+v, ok=%v, want %q", e, ok, " {Base} PV")
	}
}
