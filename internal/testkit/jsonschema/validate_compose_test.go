package jsonschema_test

import "testing"

// One negative case per composition keyword: allOf, oneOf and $ref; if/then is below.
func TestValidateCompositionKeywords(t *testing.T) {
	runKeywordCases(t, []keywordCase{
		{"allOf", `{"allOf": [{"type": "string"}, {"minLength": 5}]}`, `"ab"`, "allOf/1/minLength"},
		{"oneOf", `{"oneOf": [{"type": "string"}, {"type": "number"}]}`, `true`, "oneOf"},
		{"ref", `{"$ref": "#/$defs/pos", "$defs": {"pos": {"minimum": 0}}}`, `-1`, "$defs/pos/minimum"},
	})
}

// oneOf also fails when more than one branch matches.
func TestValidateOneOfMoreThanOne(t *testing.T) {
	schema := mustCompile(t, `{"oneOf": [{"type": "number"}, {"minimum": 0}]}`)
	if errs := schema.Validate([]byte(`1`)); len(errs) == 0 {
		t.Error("Validate(1) = no errors, want a oneOf violation (matches both branches)")
	}
	if errs := schema.Validate([]byte(`-1`)); len(errs) != 0 {
		t.Errorf("Validate(-1) = %v, want none (matches only \"number\")", errs)
	}
}

// then applies only when if holds; there is no else in this vocabulary.
func TestValidateIfThen(t *testing.T) {
	schema := mustCompile(t, `{"if": {"type": "string"}, "then": {"minLength": 3}}`)
	if errs := schema.Validate([]byte(`"ab"`)); len(errs) == 0 {
		t.Error(`Validate("ab") = no errors, want a then violation`)
	}
	if errs := schema.Validate([]byte(`1`)); len(errs) != 0 {
		t.Errorf("Validate(1) = %v, want none: if does not hold, so then is not applied", errs)
	}
}

// $ref jumps the reported keyword path to the def, not to every site that used it.
func TestValidateRefKeywordPathJumpsToDef(t *testing.T) {
	schema := mustCompile(t, `{"$ref": "#/$defs/pos", "$defs": {"pos": {"minimum": 0}}}`)
	errs := schema.Validate([]byte(`-1`))
	if len(errs) != 1 || errs[0].Keyword != "/$defs/pos/minimum" {
		t.Errorf("Validate(-1).errs = %v, want one error at /$defs/pos/minimum", errs)
	}
}

// $ref composes with a sibling keyword (2020-12 semantics, not draft-07's override): both
// apply to the same instance.
func TestValidateRefAndSiblingBothApply(t *testing.T) {
	schema := mustCompile(t, `{"$ref": "#/$defs/pos", "minimum": 100, "$defs": {"pos": {"minimum": 0}}}`)
	if errs := schema.Validate([]byte(`150`)); len(errs) != 0 {
		t.Errorf("Validate(150) = %v, want none: satisfies both minimums", errs)
	}
	errs := schema.Validate([]byte(`50`))
	if len(errs) != 1 || errs[0].Keyword != "/minimum" {
		t.Errorf("Validate(50) = %v, want one violation, the sibling's own /minimum", errs)
	}
	errs = schema.Validate([]byte(`-1`))
	if len(errs) != 2 {
		t.Errorf("Validate(-1) = %v, want two violations: the sibling and the $ref's own", errs)
	}
}

// if without then asserts nothing: if is not itself a constraint.
func TestValidateIfWithoutThen(t *testing.T) {
	schema := mustCompile(t, `{"if": {"type": "string"}}`)
	if errs := schema.Validate([]byte(`"anything"`)); len(errs) != 0 {
		t.Errorf(`Validate("anything") = %v, want none: no then to apply`, errs)
	}
	if errs := schema.Validate([]byte(`1`)); len(errs) != 0 {
		t.Errorf("Validate(1) = %v, want none: no then to apply", errs)
	}
}
