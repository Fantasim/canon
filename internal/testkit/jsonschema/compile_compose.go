package jsonschema

import "fmt"

// compileCompositionKeywords compiles allOf, oneOf, if and then.
func (c *compiler) compileCompositionKeywords(n *node, raw map[string]any, path string) error {
	allOf, err := c.compileSchemaArray(raw, kwAllOf, path)
	if err != nil {
		return err
	}
	n.allOf = allOf
	oneOf, err := c.compileSchemaArray(raw, kwOneOf, path)
	if err != nil {
		return err
	}
	n.oneOf = oneOf
	if rawIf, ok := raw[keywordName(kwIf)]; ok {
		ifNode, err := c.compileValue(rawIf, appendToken(path, keywordName(kwIf)))
		if err != nil {
			return err
		}
		n.ifs = ifNode
	}
	if rawThen, ok := raw[keywordName(kwThen)]; ok {
		then, err := c.compileValue(rawThen, appendToken(path, keywordName(kwThen)))
		if err != nil {
			return err
		}
		n.then = then
	}
	return nil
}

// compileSchemaArray compiles allOf or oneOf: a non-empty array of subschemas.
func (c *compiler) compileSchemaArray(raw map[string]any, kw keyword, path string) ([]*node, error) {
	rawKw, ok := raw[keywordName(kw)]
	if !ok {
		return nil, nil
	}
	kwPath := appendToken(path, keywordName(kw))
	rawList, ok := rawKw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: %w", kwPath, errBadArray)
	}
	if len(rawList) == 0 {
		return nil, fmt.Errorf("%s: %w", kwPath, errEmptyArray)
	}
	nodes := make([]*node, len(rawList))
	for i, item := range rawList {
		n, err := c.compileValue(item, appendIndex(kwPath, i))
		if err != nil {
			return nil, err
		}
		nodes[i] = n
	}
	return nodes, nil
}
