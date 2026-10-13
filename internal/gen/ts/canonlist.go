package tsgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// typedElem reports an element type whose literals are object literals or arrays: a record, a variant or one of its cases, a dependent value, a list or a table, or an optional of one (CODEGEN.md §8.1, DECISIONS 370).
func typedElem(t ir.TypeRef) bool {
	switch t.Kind {
	case types.Record, types.Variant, types.Case, types.TypeApp, types.List, types.Table:
		return true
	case types.Optional:
		return t.Elem != nil && typedElem(*t.Elem)
	default:
		return false
	}
}

// wrapItems writes each item as `$r(item)`, which checks it against the list's element type on its own.
func wrapItems(items []string) []string {
	out := make([]string, len(items))
	for i, x := range items {
		out[i] = fmt.Sprintf(listItemFormat, x)
	}
	return out
}

// inlineArray is `[a, b]`.
func inlineArray(items []string) string {
	return lbracket + strings.Join(items, listSep) + rbracket
}

// blockArray is the items one per line, or `[]` for none.
func blockArray(items []string) string {
	if len(items) == 0 {
		return lbracket + rbracket
	}
	return lbracket + newline + indent + strings.Join(items, listEnd+indent) + listEnd + rbracket
}

// canonListOf is canonList<elem>(($r) => array), the array holding wrapped items (CODEGEN.md §8.1, §8.2).
func (g *gen) canonListOf(elem, array string) string {
	return fmt.Sprintf(canonListFormat, g.helper(canonListName), elem, array)
}

// typedArray is an array of items typed element by element, each an object literal, tuple or array; an empty one stays `[]` (DECISIONS 370).
func (g *gen) typedArray(elem string, items []string, array func([]string) string) string {
	if len(items) == 0 {
		return array(items)
	}
	return g.canonListOf(elem, array(wrapItems(items)))
}

// frozenArray is the array deep-frozen, typed ReadonlyArray<elem>: an array of object literals goes through canonList and is frozen without a type argument, else the freeze names the type.
func (g *gen) frozenArray(elem string, items []string, objects bool, array func([]string) string) string {
	if !objects || len(items) == 0 {
		return fmt.Sprintf(freezeFormat, g.helper(canonFreezeName), fmt.Sprintf(readonlyArrayFormat, elem), array(items))
	}
	return fmt.Sprintf(callFormat, g.helper(canonFreezeName), g.typedArray(elem, items, array))
}
