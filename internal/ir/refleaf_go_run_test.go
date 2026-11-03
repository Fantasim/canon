package ir_test

import "testing"

const goRunNestedUnions = "testdata/e2e/gorun/nested_ref_unions.txtar"

// TestGoNestedRefUnionsRun is DECISIONS 387, 456 and CODEGEN.md §5.8, §5.9 built and run: a go data loader reads a union over a ref into a keyed list keyed by a ref into a keyed list keyed by an enum through the enum's Parse, as it does one level down (TestGoRefUnionsRun), and refuses a name that is not a wire and a member the list lacks with the ref check's text.
func TestGoNestedRefUnionsRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunNestedUnions)
}
