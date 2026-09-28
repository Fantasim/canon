package jsonschema

import "fmt"

// validateComposition checks allOf, oneOf, if/then and $ref, none of which cares about the
// instance's own kind: each re-enters validate on the same instance.
func (v *validator) validateComposition(n *node, instance any, instPtr, kwPtr string) []Violation {
	var errs []Violation
	errs = append(errs, v.checkAllOf(n, instance, instPtr, kwPtr)...)
	errs = append(errs, v.checkOneOf(n, instance, instPtr, kwPtr)...)
	errs = append(errs, v.checkIfThen(n, instance, instPtr, kwPtr)...)
	errs = append(errs, v.checkRef(n, instance, instPtr, kwPtr)...)
	return errs
}

// checkAllOf requires every allOf branch to hold, keeping each branch's own errors.
func (v *validator) checkAllOf(n *node, instance any, instPtr, kwPtr string) []Violation {
	var errs []Violation
	allOfPtr := appendToken(kwPtr, keywordName(kwAllOf))
	for i, sub := range n.allOf {
		errs = append(errs, v.validate(sub, instance, instPtr, appendIndex(allOfPtr, i))...)
	}
	return errs
}

// checkOneOf requires exactly one oneOf branch to hold; a branch's own errors are not
// surfaced, only the count, since neither "none" nor "more than one" points at one branch.
func (v *validator) checkOneOf(n *node, instance any, instPtr, kwPtr string) []Violation {
	if len(n.oneOf) == 0 {
		return nil
	}
	oneOfPtr := appendToken(kwPtr, keywordName(kwOneOf))
	matches := 0
	for i, sub := range n.oneOf {
		if len(v.validate(sub, instance, instPtr, appendIndex(oneOfPtr, i))) == 0 {
			matches++
		}
	}
	if matches == 1 {
		return nil
	}
	msg := fmt.Sprintf("matches %d of %d oneOf branches, want exactly one", matches, len(n.oneOf))
	return []Violation{{Instance: instPtr, Keyword: oneOfPtr, Message: msg}}
}

// checkIfThen applies then only when if holds cleanly; there is no else in this vocabulary.
func (v *validator) checkIfThen(n *node, instance any, instPtr, kwPtr string) []Violation {
	if n.ifs == nil || n.then == nil {
		return nil
	}
	if len(v.validate(n.ifs, instance, instPtr, appendToken(kwPtr, keywordName(kwIf)))) != 0 {
		return nil
	}
	return v.validate(n.then, instance, instPtr, appendToken(kwPtr, keywordName(kwThen)))
}

// checkRef validates against the named $defs entry; the keyword path jumps there, so an error
// points at the rule that actually fired rather than at every $ref site that reached it.
func (v *validator) checkRef(n *node, instance any, instPtr, _ string) []Violation {
	if n.ref == "" {
		return nil
	}
	defsPtr := appendToken("/"+keywordName(kwDefs), n.ref)
	return v.validate(v.defs[n.ref], instance, instPtr, defsPtr)
}
