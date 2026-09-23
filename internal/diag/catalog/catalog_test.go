package catalog

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

const (
	errorsPath   = "../../../spec/ERRORS.md"
	planPath     = "../../../spec/IMPLEMENTATION-PLAN.md"
	e1012Code    = "| E1012 | error | project | GRAMMAR.md §7.1 | `studio` names a package that does not exist |"
	e1012Message = "| E1012 | - | pkg:Name | `studio package \"{pkg}\" does not exist` |"
)

func readSpec(t *testing.T) (string, []byte) {
	t.Helper()
	doc, err := os.ReadFile(errorsPath)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(doc), plan
}

// ERRORS.md §2.1: the catalogue as written is accepted.
func TestParseAcceptsTheCatalogue(t *testing.T) {
	doc, plan := readSpec(t)
	c, err := Parse([]byte(doc), plan)
	if err != nil {
		t.Fatal(err)
	}
	got := countFields(c)
	if len(c.Codes) != got[0] || len(c.ArgTypes) == 0 || len(c.Kinds) == 0 || len(c.Runtime) == 0 {
		t.Errorf("parsed %d codes, %d types, %d kinds, %d runtime texts", len(c.Codes), len(c.ArgTypes), len(c.Kinds), len(c.Runtime))
	}
}

// ERRORS.md §2.1: each refusal of the generator, provoked by one edit of the catalogue.
func TestParseRefuses(t *testing.T) {
	tests := []struct {
		name, old, new string
		want           error
		all            bool
	}{
		{"code format", "| E1012 |", "| E101 |", errCode, true},
		{"code twice", e1012Code, e1012Code + "\n" + e1012Code, errCode, false},
		{"retired number", "| E3001 |", "| E3014 |", errCode, true},
		{"outside the section range", "| E1012 |", "| E2012 |", errCode, true},
		{"severity", "| E1012 | error |", "| E1012 | fatal |", errSeverity, false},
		{"letter and severity", "| W1001 | warning |", "| W1001 | error |", errSeverity, false},
		{"package not in the table", "| E1012 | error | project |", "| E1012 | error | nowhere |", errPackage, false},
		{"gen for a static code", "| E1012 | error | project |", "| E1012 | error | gen |", errPackage, false},
		{"runtime code outside gen", "| E8301 | runtime | gen |", "| E8301 | runtime | ir |", errPackage, false},
		{"no message row", e1012Message + "\n", "", errMessage, false},
		{"message row of another code", e1012Message, strings.Replace(e1012Message, "E1012", "E1013", 1), errMessage, false},
		{"unnamed variant among several", "| E1005 | key |", "| E1005 | - |", errMessage, false},
		{"named single variant", "| E1012 | - |", "| E1012 | only |", errMessage, false},
		{"variant not lowerCamel", "| E1005 | key |", "| E1005 | Key |", errMessage, false},
		{"five arguments", "| pkg:Name | `studio package \"{pkg}\"", "| pkg:Name, a:Name, b:Name, c:Name, d:Name | `{a}{b}{c}{d}studio package \"{pkg}\"", errArgs, false},
		{"reserved argument name", "| pkg:Name | `studio package \"{pkg}\"", "| type:Name | `studio package \"{type}\"", errArgs, false},
		{"span argument name", "| pkg:Name | `studio package \"{pkg}\"", "| span:Name | `studio package \"{span}\"", errArgs, false},
		{"unknown argument type", "| pkg:Name |", "| pkg:Nme |", errArgs, false},
		{"argument without type", "| pkg:Name |", "| pkg |", errArgs, false},
		{"undeclared placeholder", `"{pkg}" does not exist`, `"{pkgs}" does not exist`, errTemplate, false},
		{"unused argument", "| pkg:Name |", "| pkg:Name, other:Name |", errTemplate, false},
		{"lone closing brace", "does not exist` |", "does not exist }` |", errTemplate, false},
		{"lone opening brace", "does not exist` |", "does not exist {` |", errTemplate, false},
		{"bad escape", "does not exist` |", `does not exist \t` + "` |", errTemplate, false},
		{"pipe in template", "does not exist` |", "does not \\| exist` |", errTemplate, false},
		{"kind listing no code", "| `Array` | an array | `E7110` |", "| `Array` | an array |  |", errKind, false},
		{"kind listing a code without a Kind argument", "| `Array` | an array | `E7110` |", "| `Array` | an array | `E1012` |", errKind, false},
		{"code with a Kind argument listed by no kind", "`W1002`, ", "", errKind, true},
		{"count sentence", "\nThe catalogue holds ", "\nThe catalogue holds 1", errCount, false},
		{"runtime pair of an unknown code", "| E4108 | `clamp", "| E4109 | `clamp", errRuntime, false},
		{"missing cell", e1012Code, "| E1012 | error | project | `studio` names a package that does not exist |", errTable, false},
		{"template not a code span", "| `studio package \"{pkg}\" does not exist` |", "| studio package |", errTable, false},
	}
	doc, plan := readSpec(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(doc, tc.old) {
				t.Fatalf("ERRORS.md has no %q", tc.old)
			}
			n := 1
			if tc.all {
				n = -1
			}
			_, err := Parse([]byte(strings.Replace(doc, tc.old, tc.new, n)), plan)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// ERRORS.md §1.2: escapes are read before placeholders, left to right.
func TestPlaceholders(t *testing.T) {
	got, err := placeholders(`{{{name}}} and {{index}}, {a}\n\\ {a}`)
	if err != nil || !slices.Equal(got, []string{"name", "a", "a"}) {
		t.Errorf("got %v, %v", got, err)
	}
}

// ERRORS.md §2.1: every retired number the generator refuses is one ERRORS.md retires.
func TestRetiredCodesAreTheDocumentsOwn(t *testing.T) {
	doc, _ := readSpec(t)
	i := strings.Index(doc, "**Retired numbers**")
	j := strings.Index(doc[i:], "\n\n")
	for _, code := range retiredCodes {
		if !strings.Contains(doc[i:i+j], backtick+code+backtick) {
			t.Errorf("%s is not in the retired numbers of ERRORS.md", code)
		}
	}
}

// ERRORS.md §2.1: the same input always gives the same bytes.
func TestGenerateIsDeterministic(t *testing.T) {
	doc, plan := readSpec(t)
	c1, err1 := Parse([]byte(doc), plan)
	c2, err2 := Parse([]byte(doc), plan)
	if err := errors.Join(err1, err2); err != nil {
		t.Fatal(err)
	}
	a, err1 := c1.Generate()
	b, err2 := c2.Generate()
	if err := errors.Join(err1, err2); err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if a[i].Name != b[i].Name || !bytes.Equal(a[i].Data, b[i].Data) {
			t.Errorf("%s differs between two runs", a[i].Name)
		}
		if !bytes.HasPrefix(a[i].Data, []byte(generatedHeader)) {
			t.Errorf("%s does not start with the generated marker", a[i].Name)
		}
	}
}
