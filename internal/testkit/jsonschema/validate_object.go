package jsonschema

import "fmt"

// validateObject checks required, properties, additionalProperties, propertyNames and
// dependentRequired, when the instance is an object.
func (v *validator) validateObject(n *node, instance any, instPtr, kwPtr string) []Violation {
	obj, ok := instance.(map[string]any)
	if !ok {
		return nil
	}
	var errs []Violation
	errs = append(errs, checkRequired(n, obj, instPtr, kwPtr)...)
	errs = append(errs, v.checkProperties(n, obj, instPtr, kwPtr)...)
	errs = append(errs, v.checkPropertyNames(n, obj, instPtr, kwPtr)...)
	errs = append(errs, checkDependentRequired(n, obj, instPtr, kwPtr)...)
	return errs
}

// checkRequired reports each name of n.required absent from obj; the order is the schema's own
// (an array, already deterministic).
func checkRequired(n *node, obj map[string]any, instPtr, kwPtr string) []Violation {
	var errs []Violation
	for _, name := range n.required {
		if _, ok := obj[name]; !ok {
			msg := fmt.Sprintf("missing required property %q", name)
			kw := appendToken(kwPtr, keywordName(kwRequired))
			errs = append(errs, Violation{Instance: instPtr, Keyword: kw, Message: msg})
		}
	}
	return errs
}

// checkProperties validates each of obj's own properties against its properties entry, or,
// absent one, against additionalProperties when the keyword is present (its default is "allow
// anything", so absence checks nothing).
func (v *validator) checkProperties(n *node, obj map[string]any, instPtr, kwPtr string) []Violation {
	var errs []Violation
	propertiesPtr := appendToken(kwPtr, keywordName(kwProperties))
	additionalPtr := appendToken(kwPtr, keywordName(kwAdditionalProperties))
	for _, name := range sortedKeys(obj) {
		propPtr := appendToken(instPtr, name)
		if prop, ok := n.properties[name]; ok {
			errs = append(errs, v.validate(prop, obj[name], propPtr, appendToken(propertiesPtr, name))...)
			continue
		}
		if n.hasAdditionalProperties {
			errs = append(errs, v.validate(n.additionalProperties, obj[name], propPtr, additionalPtr)...)
		}
	}
	return errs
}

// checkPropertyNames validates every key of obj, as a string instance, against propertyNames.
func (v *validator) checkPropertyNames(n *node, obj map[string]any, instPtr, kwPtr string) []Violation {
	if n.propertyNames == nil {
		return nil
	}
	var errs []Violation
	namesPtr := appendToken(kwPtr, keywordName(kwPropertyNames))
	for _, name := range sortedKeys(obj) {
		errs = append(errs, v.validate(n.propertyNames, name, appendToken(instPtr, name), namesPtr)...)
	}
	return errs
}

// checkDependentRequired reports each dependency of n.dependentRequired left unmet.
func checkDependentRequired(n *node, obj map[string]any, instPtr, kwPtr string) []Violation {
	if n.dependentRequired == nil {
		return nil
	}
	var errs []Violation
	depPtr := appendToken(kwPtr, keywordName(kwDependentRequired))
	for _, trigger := range sortedKeys(n.dependentRequired) {
		if _, present := obj[trigger]; !present {
			continue
		}
		errs = append(errs, checkDependency(n.dependentRequired[trigger], obj, trigger, instPtr, depPtr)...)
	}
	return errs
}

// checkDependency reports each name required missing from obj, because trigger is present.
func checkDependency(names []string, obj map[string]any, trigger, instPtr, depPtr string) []Violation {
	var errs []Violation
	for _, name := range names {
		if _, ok := obj[name]; !ok {
			msg := fmt.Sprintf("%q requires property %q", trigger, name)
			errs = append(errs, Violation{Instance: instPtr, Keyword: appendToken(depPtr, trigger), Message: msg})
		}
	}
	return errs
}
