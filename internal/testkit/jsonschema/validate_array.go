package jsonschema

import "fmt"

// validateArray checks minItems, maxItems, uniqueItems and items, when the instance is an array.
func (v *validator) validateArray(n *node, instance any, instPtr, kwPtr string) []Violation {
	arr, ok := instance.([]any)
	if !ok {
		return nil
	}
	var errs []Violation
	if n.minItems != nil && len(arr) < *n.minItems {
		msg := fmt.Sprintf("want at least %d items, got %d", *n.minItems, len(arr))
		kw := appendToken(kwPtr, keywordName(kwMinItems))
		errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
	}
	if n.maxItems != nil && len(arr) > *n.maxItems {
		msg := fmt.Sprintf("want at most %d items, got %d", *n.maxItems, len(arr))
		kw := appendToken(kwPtr, keywordName(kwMaxItems))
		errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
	}
	if n.uniqueItems {
		errs = append(errs, checkUniqueItems(arr, instPtr, kwPtr)...)
	}
	if n.items != nil {
		itemsPtr := appendToken(kwPtr, keywordName(kwItems))
		for i, elem := range arr {
			errs = append(errs, v.validate(n.items, elem, appendIndex(instPtr, i), itemsPtr)...)
		}
	}
	return errs
}

// checkUniqueItems reports each item equal to an earlier one (2020-12 §6.4.3, JSON equality).
func checkUniqueItems(arr []any, instPtr, kwPtr string) []Violation {
	var errs []Violation
	for i := 1; i < len(arr); i++ {
		for j := 0; j < i; j++ {
			if equalJSON(arr[i], arr[j]) {
				msg := fmt.Sprintf("item %d equals item %d", i, j)
				kw := appendToken(kwPtr, keywordName(kwUniqueItems))
				errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
				break
			}
		}
	}
	return errs
}
