package std

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Host is what a built-in needs from the evaluator. Invoke and Charge return false once the
// root is aborted (a hard error, a poisoned read, the budget); the built-in then returns false.
type Host interface {
	Invoke(fn value.Value, args ...value.Value) (value.Value, bool)
	Charge(n int) bool
	// Remaining is the steps the invocation may still spend before its budget runs out.
	Remaining() int
	// Fail reports a hard error (EVALUATION.md §7.1) and aborts the root.
	Fail(b *diag.Builder)
	// Site is the call expression, where STDLIB.md §1.4 locates every error of a built-in.
	Site() source.Span
	// Entry is the entry of key k of a table or keyed list.
	Entry(coll value.Value, k value.Key) (*value.Record, bool)
	Regexp(pattern string) *regexp.Regexp
	// Equal is value equality, a step per composite pair visited (DECISIONS 197).
	Equal(a, b value.Value) (equal, ok bool)
	// Coerce converts a value to type t as a storage point does (TYPES.md §6.2).
	Coerce(v value.Value, t types.Type) (value.Value, bool)
}

// Call is one call of a built-in: its name, the row of its signature table that matched
// (check.Callee.Overload), its receiver (nil for a free function), its arguments in parameter
// order, its static result type and the provenance of what it computes.
type Call struct {
	Name     string
	Overload int
	Recv     value.Value
	Args     []value.Value
	Result   types.Type
	Prov     *value.Prov
}

type builtin func(h Host, c *Call) (value.Value, bool)

// family is the receiver kind a method table serves (STDLIB.md §1.2).
type family uint8

// Method runs the built-in method c.Name on c.Recv (STDLIB.md §4 to §7, §10).
func Method(h Host, c *Call) (value.Value, bool) {
	fn := methods[familyOf(c.Recv)][c.Name]
	if fn == nil {
		return nil, false
	}
	return fn(h, c)
}

// Free runs the free function c.Name (STDLIB.md §2).
func Free(h Host, c *Call) (value.Value, bool) {
	fn := freeFunctions[c.Name]
	if fn == nil {
		return nil, false
	}
	return fn(h, c)
}

// ParamIndex is the position of parameter param of built-in name on recv, -1 if none (STDLIB.md §1.1).
func ParamIndex(recv value.Value, name, param string) int {
	f := famFree
	if recv != nil {
		f = familyOf(recv)
	}
	for i, p := range paramNames[f][name] {
		if p == param {
			return i
		}
	}
	return -1
}

// familyOf is the method table of a receiver: a keyed collection, a plain list, a map, a
// string or a range.
func familyOf(v value.Value) family {
	switch x := v.(type) {
	case *value.Table:
		return famKeyed
	case *value.List:
		if Keyed(x) {
			return famKeyed
		}
		return famList
	case *value.Map:
		return famMap
	case *value.Str:
		return famString
	case *value.Range:
		return famRange
	}
	return famFree
}

// Keyed reports a keyed list (TYPES.md §9.1).
func Keyed(l *value.List) bool {
	lt, ok := l.T.Base().(*types.ListType)
	return ok && lt.KeyedBy != nil
}
