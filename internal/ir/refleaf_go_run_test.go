package ir_test

import "testing"

const goRunNestedUnions = "testdata/e2e/gorun/nested_ref_unions.txtar"

// TestGoNestedRefUnionsRun is DECISIONS 387, 456 and CODEGEN.md §5.8, §5.9 built and run: a go data loader reads a union over a ref into a keyed list keyed by a ref into a keyed list keyed by an enum through the enum's Parse, as it does one level down (TestGoRefUnionsRun), and refuses a name that is not a wire and a member the list lacks with the ref check's text.
func TestGoNestedRefUnionsRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunNestedUnions)
}

const goRunNestedRefs = "testdata/e2e/gorun/nested_refs.txtar"

// TestGoNestedRefsRun is DECISIONS 456 and CODEGEN.md §5.6, §5.8 built and run: a go emit holds a ref over a chain of refs ending in an enum at that enum, baked, data (with and without @reload) and types: its key read off the entries, its branch table keyed by the enum, its loader's refusals worded by the key that ends the chain.
func TestGoNestedRefsRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunNestedRefs)
}
