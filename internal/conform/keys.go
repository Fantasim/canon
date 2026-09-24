package conform

import (
	"cmp"
	"math"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyOf is v's key in a set deduplicated by value, floats by bits, a case by its kind (CONFORMANCE.md §6).
func keyOf(v value.Value) (string, bool) {
	switch x := v.(type) {
	case *value.Int:
		return keyInt + strconv.FormatInt(x.V, decBase) + keyEnd, true
	case *value.Dur:
		return keyDur + strconv.FormatInt(x.Ms, decBase) + keyEnd, true
	case *value.Float:
		return keyFloat + strconv.FormatUint(math.Float64bits(x.V), hexBase) + keyEnd, true
	case *value.Str:
		return keyStr + strconv.Itoa(len(x.V)) + keyLen + x.V + keyEnd, true
	case *value.Bool:
		return keyBool + strconv.FormatBool(x.V) + keyEnd, true
	case *value.Member:
		return keyMember + strconv.Itoa(x.Index) + keyEnd, true
	case *value.None:
		return keyNone + keyEnd, true
	case *value.Record:
		if c, ok := x.T.Base().(*types.CaseType); ok {
			return keyCase + strconv.Itoa(c.Index) + keyEnd, true
		}
	}
	return "", false
}

// keysOf is the key of a tuple of values; false when one has none.
func keysOf(vs []value.Value) (string, bool) {
	var b strings.Builder
	for _, v := range vs {
		k, ok := keyOf(v)
		if !ok {
			return "", false
		}
		b.WriteString(k)
	}
	return b.String(), true
}

// compareFloat orders numbers ascending, -0.0 before 0.0 (CONFORMANCE.md §6.2).
func compareFloat(a, b float64) int {
	if c := cmp.Compare(a, b); c != 0 {
		return c
	}
	return -cmp.Compare(boolRank(math.Signbit(a)), boolRank(math.Signbit(b)))
}

// boolRank orders false before true.
func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}
