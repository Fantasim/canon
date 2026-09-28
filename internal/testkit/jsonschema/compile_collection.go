package jsonschema

import "fmt"

// compileArrayKeywords compiles items, minItems, maxItems and uniqueItems.
func (c *compiler) compileArrayKeywords(n *node, raw map[string]any, path string) error {
	if rawItems, ok := raw[keywordName(kwItems)]; ok {
		items, err := c.compileValue(rawItems, appendToken(path, keywordName(kwItems)))
		if err != nil {
			return err
		}
		n.items = items
	}
	if rawMin, ok := raw[keywordName(kwMinItems)]; ok {
		count, err := jsonInt(rawMin)
		if err != nil {
			return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwMinItems)), err)
		}
		n.minItems = &count
	}
	if rawMax, ok := raw[keywordName(kwMaxItems)]; ok {
		count, err := jsonInt(rawMax)
		if err != nil {
			return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwMaxItems)), err)
		}
		n.maxItems = &count
	}
	rawUnique, ok := raw[keywordName(kwUniqueItems)]
	if !ok {
		return nil
	}
	unique, ok := rawUnique.(bool)
	if !ok {
		return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwUniqueItems)), errBadBool)
	}
	n.uniqueItems = unique
	return nil
}

// compileObjectKeywords compiles properties, additionalProperties, propertyNames, required and
// dependentRequired.
func (c *compiler) compileObjectKeywords(n *node, raw map[string]any, path string) error {
	if err := c.compileProperties(n, raw, path); err != nil {
		return err
	}
	if err := c.compileAdditionalProperties(n, raw, path); err != nil {
		return err
	}
	if rawNames, ok := raw[keywordName(kwPropertyNames)]; ok {
		names, err := c.compileValue(rawNames, appendToken(path, keywordName(kwPropertyNames)))
		if err != nil {
			return err
		}
		n.propertyNames = names
	}
	if err := c.compileRequired(n, raw, path); err != nil {
		return err
	}
	return c.compileDependentRequired(n, raw, path)
}

func (c *compiler) compileProperties(n *node, raw map[string]any, path string) error {
	rawProps, ok := raw[keywordName(kwProperties)]
	if !ok {
		return nil
	}
	propsPath := appendToken(path, keywordName(kwProperties))
	propsMap, ok := rawProps.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: %w", propsPath, errBadObject)
	}
	n.properties = make(map[string]*node, len(propsMap))
	for _, name := range sortedKeys(propsMap) {
		prop, err := c.compileValue(propsMap[name], appendToken(propsPath, name))
		if err != nil {
			return err
		}
		n.properties[name] = prop
	}
	return nil
}

func (c *compiler) compileAdditionalProperties(n *node, raw map[string]any, path string) error {
	rawAP, ok := raw[keywordName(kwAdditionalProperties)]
	if !ok {
		return nil
	}
	ap, err := c.compileValue(rawAP, appendToken(path, keywordName(kwAdditionalProperties)))
	if err != nil {
		return err
	}
	n.additionalProperties = ap
	n.hasAdditionalProperties = true
	return nil
}

// compileRequired compiles required: an array of unique strings (2020-12 §6.5.3).
func (c *compiler) compileRequired(n *node, raw map[string]any, path string) error {
	rawReq, ok := raw[keywordName(kwRequired)]
	if !ok {
		return nil
	}
	kwPath := appendToken(path, keywordName(kwRequired))
	list, ok := rawReq.([]any)
	if !ok {
		return fmt.Errorf("%s: %w", kwPath, errBadArray)
	}
	req, err := stringSlice(list, kwPath)
	if err != nil {
		return err
	}
	if err := checkNoDuplicateStrings(req, kwPath); err != nil {
		return err
	}
	n.required = req
	return nil
}

// compileDependentRequired compiles dependentRequired: an object of arrays of strings.
func (c *compiler) compileDependentRequired(n *node, raw map[string]any, path string) error {
	rawDep, ok := raw[keywordName(kwDependentRequired)]
	if !ok {
		return nil
	}
	depPath := appendToken(path, keywordName(kwDependentRequired))
	depMap, ok := rawDep.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: %w", depPath, errBadObject)
	}
	n.dependentRequired = make(map[string][]string, len(depMap))
	for _, name := range sortedKeys(depMap) {
		list, ok := depMap[name].([]any)
		if !ok {
			return fmt.Errorf("%s: %w", appendToken(depPath, name), errBadArray)
		}
		values, err := stringSlice(list, appendToken(depPath, name))
		if err != nil {
			return err
		}
		n.dependentRequired[name] = values
	}
	return nil
}

// stringSlice converts a decodeJSON array to []string, or fails on a non-string element.
func stringSlice(raw []any, path string) ([]string, error) {
	out := make([]string, len(raw))
	for i, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s: %w", appendIndex(path, i), errBadStringList)
		}
		out[i] = s
	}
	return out, nil
}
