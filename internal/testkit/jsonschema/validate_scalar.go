package jsonschema

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// validateType checks type; absent means any kind, per 2020-12 §6.1.1.
func (v *validator) validateType(n *node, instance any, instPtr, kwPtr string) []Violation {
	if len(n.types) == 0 {
		return nil
	}
	for _, t := range n.types {
		if satisfiesType(instance, t) {
			return nil
		}
	}
	msg := fmt.Sprintf("want type %s, got %s", strings.Join(n.types, " or "), jsonKind(instance))
	return []Violation{{Instance: instPtr, Keyword: appendToken(kwPtr, keywordName(kwType)), Message: msg}}
}

// validateConst checks const, by JSON equality (§4.2.3).
func (v *validator) validateConst(n *node, instance any, instPtr, kwPtr string) []Violation {
	if !n.hasConst || equalJSON(instance, n.constVal) {
		return nil
	}
	kw := appendToken(kwPtr, keywordName(kwConst))
	return []Violation{{Instance: instPtr, Keyword: kw, Message: "does not equal the const value"}}
}

// validateEnum checks enum, by JSON equality.
func (v *validator) validateEnum(n *node, instance any, instPtr, kwPtr string) []Violation {
	if n.enumVals == nil {
		return nil
	}
	for _, e := range n.enumVals {
		if equalJSON(instance, e) {
			return nil
		}
	}
	kw := appendToken(kwPtr, keywordName(kwEnum))
	return []Violation{{Instance: instPtr, Keyword: kw, Message: "is not one of the enum values"}}
}

// validateString checks minLength (Unicode code points, not bytes) and pattern (an unanchored
// regexp search, STD-03), when the instance is a string; otherwise these keywords are vacuous.
func (v *validator) validateString(n *node, instance any, instPtr, kwPtr string) []Violation {
	s, ok := instance.(string)
	if !ok {
		return nil
	}
	var errs []Violation
	if n.minLength != nil {
		if length := utf8.RuneCountInString(s); length < *n.minLength {
			msg := fmt.Sprintf("want at least %d code points, got %d", *n.minLength, length)
			kw := appendToken(kwPtr, keywordName(kwMinLength))
			errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
		}
	}
	// pattern is 2020-12's ECMA-262 regex (§6.3.3); Go's RE2 (STD-03) is a deliberate substitute.
	if n.pattern != nil && !n.pattern.MatchString(s) {
		msg := fmt.Sprintf("does not match %q", n.pattern.String())
		kw := appendToken(kwPtr, keywordName(kwPattern))
		errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
	}
	return errs
}

// validateNumber checks minimum and maximum, compared exactly, when the instance is a number.
// A number whose exponent is too large for toRat to parse exactly is reported as a violation
// of any bound this node declares, rather than silently passing it.
func (v *validator) validateNumber(n *node, instance any, instPtr, kwPtr string) []Violation {
	num, ok := instance.(json.Number)
	if !ok {
		return nil
	}
	r, ok := toRat(num)
	if !ok {
		return unrepresentableNumber(n, instPtr, kwPtr)
	}
	var errs []Violation
	if n.minimum != nil && r.Cmp(n.minimum) < 0 {
		msg := fmt.Sprintf("want >= %s, got %s", n.minimum.RatString(), r.RatString())
		kw := appendToken(kwPtr, keywordName(kwMinimum))
		errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
	}
	if n.maximum != nil && r.Cmp(n.maximum) > 0 {
		msg := fmt.Sprintf("want <= %s, got %s", n.maximum.RatString(), r.RatString())
		kw := appendToken(kwPtr, keywordName(kwMaximum))
		errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
	}
	return errs
}

// unrepresentableNumber reports minimum and maximum, if this node declares either, as unmet:
// a bound that cannot be checked exactly is not a bound that passes.
func unrepresentableNumber(n *node, instPtr, kwPtr string) []Violation {
	var errs []Violation
	if n.minimum != nil {
		kw := appendToken(kwPtr, keywordName(kwMinimum))
		errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msgNotRepresentable})
	}
	if n.maximum != nil {
		kw := appendToken(kwPtr, keywordName(kwMaximum))
		errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msgNotRepresentable})
	}
	return errs
}
