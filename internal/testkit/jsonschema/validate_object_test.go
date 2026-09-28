package jsonschema_test

import "testing"

// One negative case per object keyword: required, properties, additionalProperties,
// propertyNames and dependentRequired.
func TestValidateObjectKeywords(t *testing.T) {
	runKeywordCases(t, []keywordCase{
		{"required", `{"required": ["a"]}`, `{}`, "required"},
		{"properties", `{"properties": {"a": {"type": "string"}}}`, `{"a": 1}`, "properties/a/type"},
		{"additionalProperties", `{"additionalProperties": false}`, `{"a": 1}`, "additionalProperties"},
		{"propertyNames", `{"propertyNames": {"pattern": "^[a-z]+$"}}`, `{"A": 1}`, "propertyNames/pattern"},
		{"dependentRequired", `{"dependentRequired": {"a": ["b"]}}`, `{"a": 1}`, "dependentRequired/a"},
	})
}

// additionalProperties defaults to "allow anything" when the keyword is absent.
func TestValidateAdditionalPropertiesAbsentAllowsExtra(t *testing.T) {
	schema := mustCompile(t, `{"properties": {"a": {"type": "string"}}}`)
	if errs := schema.Validate([]byte(`{"a": "x", "b": 1}`)); len(errs) != 0 {
		t.Errorf("Validate with an extra property = %v, want none (no additionalProperties)", errs)
	}
}

// A property listed in properties is exempt from additionalProperties, even when false.
func TestValidateAdditionalPropertiesExemptsProperties(t *testing.T) {
	schema := mustCompile(t, `{"properties": {"a": {"type": "string"}}, "additionalProperties": false}`)
	if errs := schema.Validate([]byte(`{"a": "x"}`)); len(errs) != 0 {
		t.Errorf("Validate({a: x}) = %v, want none", errs)
	}
	if errs := schema.Validate([]byte(`{"a": "x", "b": 1}`)); len(errs) == 0 {
		t.Error("Validate with an extra property = no errors, want an additionalProperties violation")
	}
}

// dependentRequired only fires when the trigger property is present.
func TestValidateDependentRequiredVacuousWithoutTrigger(t *testing.T) {
	schema := mustCompile(t, `{"dependentRequired": {"a": ["b"]}}`)
	if errs := schema.Validate([]byte(`{}`)); len(errs) != 0 {
		t.Errorf("Validate({}) = %v, want none: trigger absent", errs)
	}
}

// dependentRequired reports only the names still missing, once the trigger fires.
func TestValidateDependentRequiredPartial(t *testing.T) {
	schema := mustCompile(t, `{"dependentRequired": {"a": ["b", "c"]}}`)
	errs := schema.Validate([]byte(`{"a": 1, "b": 2}`))
	if len(errs) != 1 || errs[0].Keyword != "/dependentRequired/a" {
		t.Errorf(`Validate({a,b}) = %v, want one violation naming "c"`, errs)
	}
	if errs := schema.Validate([]byte(`{"a": 1, "b": 2, "c": 3}`)); len(errs) != 0 {
		t.Errorf("Validate({a,b,c}) = %v, want none: both dependencies met", errs)
	}
}

// required, properties and additionalProperties are vacuous on a non-object instance.
func TestValidateObjectKeywordsVacuousOnNonObject(t *testing.T) {
	schema := mustCompile(t, `{"required": ["a"], "additionalProperties": false}`)
	if errs := schema.Validate([]byte(`"not an object"`)); len(errs) != 0 {
		t.Errorf("Validate on a non-object instance = %v, want none", errs)
	}
}
