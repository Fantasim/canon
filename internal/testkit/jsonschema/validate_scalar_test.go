package jsonschema_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// keywordCase is one negative case: schema, an instance that must fail it, and the keyword's
// own name, checked against the reported Violation.Keyword suffix.
type keywordCase struct {
	name, schema, doc, keyword string
}

func runKeywordCases(t *testing.T, cases []keywordCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema, err := jsonschema.Compile([]byte(tc.schema))
			if err != nil {
				t.Fatalf("Compile(%s) = %v, want nil", tc.schema, err)
			}
			errs := schema.Validate([]byte(tc.doc))
			if len(errs) == 0 {
				t.Fatalf("Validate(%s) = no errors, want a %s violation", tc.doc, tc.keyword)
			}
			found := false
			for _, e := range errs {
				if strings.HasSuffix(e.Keyword, "/"+tc.keyword) {
					found = true
				}
			}
			if !found {
				t.Errorf("Validate(%s) = %v, want a /%s error", tc.doc, errs, tc.keyword)
			}
		})
	}
}

// One negative case per scalar keyword: type, const, enum, minLength (code points, not bytes),
// pattern (an unanchored search), minimum and maximum (compared exactly).
func TestValidateScalarKeywords(t *testing.T) {
	runKeywordCases(t, []keywordCase{
		{"type", `{"type": "string"}`, `1`, "type"},
		{"const", `{"const": "a"}`, `"b"`, "const"},
		{"enum", `{"enum": ["a", "b"]}`, `"c"`, "enum"},
		{"minLength", `{"type": "string", "minLength": 3}`, `"ab"`, "minLength"},
		{"pattern", `{"type": "string", "pattern": "^[a-z]+$"}`, `"AB"`, "pattern"},
		{"minimum", `{"minimum": 5}`, `4`, "minimum"},
		{"maximum", `{"maximum": 5}`, `6`, "maximum"},
	})
}

// minLength counts Unicode code points, not UTF-8 bytes: "é" (one code point, two bytes)
// satisfies minLength: 1.
func TestValidateMinLengthCodePoints(t *testing.T) {
	schema, err := jsonschema.Compile([]byte(`{"type": "string", "minLength": 1}`))
	if err != nil {
		t.Fatalf("Compile = %v, want nil", err)
	}
	if errs := schema.Validate([]byte(`"é"`)); len(errs) != 0 {
		t.Errorf("Validate(\"é\") = %v, want none (1 code point)", errs)
	}
	if errs := schema.Validate([]byte(`""`)); len(errs) == 0 {
		t.Error("Validate(\"\") = no errors, want a minLength violation")
	}
}

// const and enum compare numbers exactly: 1, 1.0 and 1e0 are the same JSON number.
func TestValidateConstEnumNumericEquality(t *testing.T) {
	schema, err := jsonschema.Compile([]byte(`{"const": 1.0}`))
	if err != nil {
		t.Fatalf("Compile = %v, want nil", err)
	}
	for _, doc := range []string{`1`, `1.0`, `1e0`} {
		if errs := schema.Validate([]byte(doc)); len(errs) != 0 {
			t.Errorf("Validate(%s) = %v, want none (equals const 1.0)", doc, errs)
		}
	}
}

// minimum/maximum on a number too large for float64 to represent exactly must still compare
// correctly: math/big, never float64.
func TestValidateBigNumberExact(t *testing.T) {
	schema, err := jsonschema.Compile([]byte(`{"maximum": 9007199254740993}`))
	if err != nil {
		t.Fatalf("Compile = %v, want nil", err)
	}
	if errs := schema.Validate([]byte(`9007199254740993`)); len(errs) != 0 {
		t.Errorf("Validate(max) = %v, want none: equal to the maximum", errs)
	}
	if errs := schema.Validate([]byte(`9007199254740994`)); len(errs) == 0 {
		t.Error("Validate(max+1) = no errors, want a maximum violation")
	}
}

// pattern is an unanchored search (STD-03): a match anywhere in the string satisfies it.
func TestValidatePatternUnanchored(t *testing.T) {
	schema, err := jsonschema.Compile([]byte(`{"type": "string", "pattern": "abc"}`))
	if err != nil {
		t.Fatalf("Compile = %v, want nil", err)
	}
	if errs := schema.Validate([]byte(`"xxabcxx"`)); len(errs) != 0 {
		t.Errorf(`Validate("xxabcxx") = %v, want none: "abc" occurs in the string`, errs)
	}
	if errs := schema.Validate([]byte(`"xyz"`)); len(errs) == 0 {
		t.Error(`Validate("xyz") = no errors, want a pattern violation`)
	}
}

// An instance number whose exponent is too large to parse exactly is a violation of any bound
// declared, not a silent pass.
func TestValidateNumberNotRepresentable(t *testing.T) {
	schema, err := jsonschema.Compile([]byte(`{"minimum": 0, "maximum": 10}`))
	if err != nil {
		t.Fatalf("Compile = %v, want nil", err)
	}
	errs := schema.Validate([]byte(`1e400000000`))
	if len(errs) != 2 {
		t.Fatalf("Validate(1e400000000) = %v, want a minimum and a maximum violation", errs)
	}
}

// type: "integer" requires zero fractional part, exactly, not float64 rounding.
func TestValidateIntegerType(t *testing.T) {
	schema, err := jsonschema.Compile([]byte(`{"type": "integer"}`))
	if err != nil {
		t.Fatalf("Compile = %v, want nil", err)
	}
	if errs := schema.Validate([]byte(`4.0`)); len(errs) != 0 {
		t.Errorf("Validate(4.0) = %v, want none: whole number", errs)
	}
	if errs := schema.Validate([]byte(`4.5`)); len(errs) == 0 {
		t.Error("Validate(4.5) = no errors, want a type violation")
	}
}
