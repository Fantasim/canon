package gogen_test

import (
	"regexp"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
)

// A case's @go(name:) override is the whole case type, verbatim; As<Name> and the kind member
// substitute it for UpperCamel(c); a retired case's kind member gets the Retired. doc line.
func TestCaseOverrideAndRetired(t *testing.T) {
	v := &ir.Variant{Pkg: "p", Name: "Shape", Cases: []*ir.Case{
		{Name: "circle", Wire: "circle", Go: ir.NameOptions{Name: "Round"}, Fields: []*ir.Field{{Name: "r", Type: intT}}},
		{Name: "square", Wire: "square", Retired: true, Fields: []*ir.Field{{Name: "side", Type: intT}}},
	}}
	files, err := gogen.Generate(pkg(v), goEmit())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	src := string(files[1].Content)
	for _, want := range []string{
		"type Round struct",                             // the case type: the override, verbatim
		"func (self *Shape) AsRound() (*Round, bool) {", // As<Name> substitutes it for UpperCamel(c)
		"ShapeKindRound ShapeKind = 0",                  // the kind member, likewise substituted
		"ShapeKindSquare ShapeKind = 1",                 // an unoverridden case keeps T + UpperCamel(c)
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated source is missing %q:\n%s", want, src)
		}
	}
	if !regexp.MustCompile(`(?s)// ShapeKindSquare: Retired\.\n\s*ShapeKindSquare\b`).MatchString(src) {
		t.Errorf("the retired case's kind member is missing its Retired. doc line:\n%s", src)
	}
}
