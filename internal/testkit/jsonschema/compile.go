package jsonschema

import "fmt"

// compiler carries the state one Compile call threads through the recursive descent: the
// $defs namespace ($defs is refused anywhere but the root, so this is that one map) and the
// $ref names used, checked against it once compilation finishes.
type compiler struct {
	defs     map[string]*node
	refNames []string
}

// Compile parses and checks schemaBytes, the whole of doc.go's keyword subset.
func Compile(schemaBytes []byte) (*Schema, error) {
	raw, err := decodeJSON(schemaLabel, schemaBytes)
	if err != nil {
		return nil, err
	}
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return nil, errBadTopLevel
	}
	c := &compiler{defs: map[string]*node{}}
	root, err := c.compileValue(rawMap, rootPointer)
	if err != nil {
		return nil, err
	}
	for _, name := range c.refNames {
		if _, ok := c.defs[name]; !ok {
			return nil, fmt.Errorf("%s%s: %w", refPrefix, name, errUndefinedRef)
		}
	}
	if err := c.checkCycles(); err != nil {
		return nil, err
	}
	return &Schema{root: root, defs: c.defs}, nil
}

// compileValue compiles one schema slot: a boolean schema or a keyword object.
func (c *compiler) compileValue(raw any, path string) (*node, error) {
	switch v := raw.(type) {
	case bool:
		return &node{boolSchema: &v}, nil
	case map[string]any:
		return c.compileNode(v, path)
	default:
		return nil, fmt.Errorf("%s: %w", path, errNotASchema)
	}
}

// compileNode compiles a keyword object: every key allowed, $defs/$id/$schema root-only (path
// == rootPointer, which no other node's path equals), then each keyword group fills the node.
func (c *compiler) compileNode(raw map[string]any, path string) (*node, error) {
	for _, k := range sortedKeys(raw) {
		kw, ok := keywordOf[k]
		if !ok {
			return nil, fmt.Errorf("%s: %q: %w", path, k, errUnknownKeyword)
		}
		if path != rootPointer && rootOnlyKeywords[kw] {
			return nil, fmt.Errorf("%s: %q: %w", path, k, errRootOnlyKeyword)
		}
	}
	n := &node{}
	steps := []func(*node, map[string]any, string) error{
		c.compileDefs, c.compileSchemaKeyword, c.compileRef, c.compileType, c.compileConstEnum,
		c.compileStringKeywords, c.compileNumberKeywords, c.compileArrayKeywords,
		c.compileObjectKeywords, c.compileCompositionKeywords, c.compileAnnotations,
	}
	for _, step := range steps {
		if err := step(n, raw, path); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// compileDefs compiles $defs, if present (only reachable at the root: compileNode above
// refuses it elsewhere), into the compiler's one namespace.
func (c *compiler) compileDefs(_ *node, raw map[string]any, path string) error {
	rawDefs, ok := raw[keywordName(kwDefs)]
	if !ok {
		return nil
	}
	defsPath := appendToken(path, keywordName(kwDefs))
	defsMap, ok := rawDefs.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: %w", defsPath, errNotASchema)
	}
	for _, name := range sortedKeys(defsMap) {
		defNode, err := c.compileValue(defsMap[name], appendToken(defsPath, name))
		if err != nil {
			return err
		}
		c.defs[name] = defNode
	}
	return nil
}

// compileSchemaKeyword checks "$schema", only reachable at the root: it must name draft 2020-12.
func (c *compiler) compileSchemaKeyword(_ *node, raw map[string]any, path string) error {
	rawSchema, ok := raw[keywordName(kwSchema)]
	if !ok {
		return nil
	}
	if s, ok := rawSchema.(string); ok && s == draft202012URI {
		return nil
	}
	return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwSchema)), errBadSchemaURI)
}

// compileRef compiles $ref: only "#/$defs/<name>" is accepted; existence is checked once, in
// Compile, after every $defs entry has been seen.
func (c *compiler) compileRef(n *node, raw map[string]any, path string) error {
	rawRef, ok := raw[keywordName(kwRef)]
	if !ok {
		return nil
	}
	ref, ok := rawRef.(string)
	if !ok || len(ref) <= len(refPrefix) || ref[:len(refPrefix)] != refPrefix {
		return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwRef)), errNonLocalRef)
	}
	n.ref = ref[len(refPrefix):]
	c.refNames = append(c.refNames, n.ref)
	return nil
}
