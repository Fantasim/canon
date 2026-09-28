package jsonschema

import "fmt"

// validator carries the one thing every keyword validator needs beyond its arguments: the
// compiled $defs, for $ref (validate_compose.go).
type validator struct {
	defs map[string]*node
}

// checkFn is one keyword group's check, threaded through validate's table (compile.go's
// compileNode uses the same shape for its steps).
type checkFn func(*validator, *node, any, string, string) []Violation

// checks runs in this fixed order, always the same, for a deterministic error order. A
// function, not a package var: validate (below) reaches it indirectly through validateArray,
// validateObject and validateComposition, so a package var would be an init cycle.
func checks() []checkFn {
	return []checkFn{
		(*validator).validateType,
		(*validator).validateConst,
		(*validator).validateEnum,
		(*validator).validateString,
		(*validator).validateNumber,
		(*validator).validateArray,
		(*validator).validateObject,
		(*validator).validateComposition,
	}
}

// Validate checks docBytes against s, returning every failed assertion; nil means it validates.
func (s *Schema) Validate(docBytes []byte) []Violation {
	instance, err := decodeJSON(documentLabel, docBytes)
	if err != nil {
		return []Violation{{Instance: rootPointer, Keyword: rootPointer, Message: fmt.Sprintf("invalid JSON: %v", err)}}
	}
	v := &validator{defs: s.defs}
	return v.validate(s.root, instance, rootPointer, rootPointer)
}

// validate checks instance against n, at instance pointer instPtr and schema pointer kwPtr.
func (v *validator) validate(n *node, instance any, instPtr, kwPtr string) []Violation {
	if n.boolSchema != nil {
		if *n.boolSchema {
			return nil
		}
		return []Violation{{Instance: instPtr, Keyword: kwPtr, Message: "no instance satisfies a false schema"}}
	}
	var errs []Violation
	for _, check := range checks() {
		errs = append(errs, check(v, n, instance, instPtr, kwPtr)...)
	}
	return errs
}
