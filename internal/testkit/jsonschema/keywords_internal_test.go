package jsonschema

import "testing"

// keywordNames must hold exactly one non-empty spelling per keyword, in step with keywordOf:
// a keyword keywordOf never sets, or a duplicate, would name it "" (keywordName never panics)
// silently otherwise.
func TestKeywordNamesComplete(t *testing.T) {
	if len(keywordOf) != int(kwCount) {
		t.Fatalf("len(keywordOf) = %d, want %d (kwCount)", len(keywordOf), kwCount)
	}
	for k := keyword(0); k < kwCount; k++ {
		if keywordName(k) == "" {
			t.Errorf("keywordName(%d) = \"\", want a non-empty spelling", k)
		}
	}
}
