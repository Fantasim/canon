package jsonschema_test

import "testing"

// One negative case per array keyword: minItems, maxItems, uniqueItems and items (2020-12 §6.4).
func TestValidateArrayKeywords(t *testing.T) {
	runKeywordCases(t, []keywordCase{
		{"minItems", `{"minItems": 2}`, `[1]`, "minItems"},
		{"maxItems", `{"maxItems": 1}`, `[1, 2]`, "maxItems"},
		{"uniqueItems", `{"uniqueItems": true}`, `[1, 1]`, "uniqueItems"},
		{"items", `{"items": {"type": "string"}}`, `[1]`, "items/type"},
	})
}

// uniqueItems compares by JSON equality, so 1 and 1.0 are the same item.
func TestValidateUniqueItemsNumericEquality(t *testing.T) {
	schema := mustCompile(t, `{"uniqueItems": true}`)
	if errs := schema.Validate([]byte(`[1, 1.0]`)); len(errs) == 0 {
		t.Error("Validate([1, 1.0]) = no errors, want a uniqueItems violation")
	}
	if errs := schema.Validate([]byte(`[1, 2]`)); len(errs) != 0 {
		t.Errorf("Validate([1, 2]) = %v, want none", errs)
	}
}

// items applies only when the instance is an array: a non-array instance is vacuously fine.
func TestValidateItemsVacuousOnNonArray(t *testing.T) {
	schema := mustCompile(t, `{"items": {"type": "string"}}`)
	if errs := schema.Validate([]byte(`"not an array"`)); len(errs) != 0 {
		t.Errorf("Validate on a non-array instance = %v, want none", errs)
	}
}
