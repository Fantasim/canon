package live

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TYPES.md §8.4, DECISIONS 304: substituting through a `past` Refined keeps its Past.
func TestSubstituteKeepsPast(t *testing.T) {
	in := &types.Refined{Of: types.StringType, Past: true}
	got, ok := (&session{}).substitute(in, nil)
	r, isRefined := got.(*types.Refined)
	if !ok || !isRefined || !r.Past {
		t.Fatalf("substitute(%s) = %#v, %v; want a past Refined", in, got, ok)
	}
}
