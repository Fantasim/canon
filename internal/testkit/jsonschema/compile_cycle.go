package jsonschema

import "fmt"

// checkCycles refuses a $ref/allOf/oneOf/if/then chain that revisits a def without ever
// checking a different part of the instance: Validate would recurse forever on it. Recursion
// through items, properties, additionalProperties or propertyNames is fine (walkSameInstance).
func (c *compiler) checkCycles() error {
	visiting := map[string]bool{}
	done := map[string]bool{}
	for _, name := range sortedKeys(c.defs) {
		if err := c.walkDef(name, visiting, done); err != nil {
			return err
		}
	}
	return nil
}

// walkDef enters def name once (memoized in done), reporting a cycle when it is already on
// the current path (visiting).
func (c *compiler) walkDef(name string, visiting, done map[string]bool) error {
	if done[name] {
		return nil
	}
	if visiting[name] {
		return fmt.Errorf("%s%s: %w", refPrefix, name, errRefCycle)
	}
	visiting[name] = true
	if err := c.walkSameInstance(c.defs[name], visiting, done); err != nil {
		return err
	}
	visiting[name] = false
	done[name] = true
	return nil
}

// walkSameInstance follows every edge that checks the same instance value as n: $ref, each
// allOf and oneOf branch, if and then.
func (c *compiler) walkSameInstance(n *node, visiting, done map[string]bool) error {
	if n == nil || n.boolSchema != nil {
		return nil
	}
	if n.ref != "" {
		if err := c.walkDef(n.ref, visiting, done); err != nil {
			return err
		}
	}
	if err := c.walkList(n.allOf, visiting, done); err != nil {
		return err
	}
	if err := c.walkList(n.oneOf, visiting, done); err != nil {
		return err
	}
	if err := c.walkSameInstance(n.ifs, visiting, done); err != nil {
		return err
	}
	return c.walkSameInstance(n.then, visiting, done)
}

// walkList runs walkSameInstance over every branch of an allOf or oneOf.
func (c *compiler) walkList(list []*node, visiting, done map[string]bool) error {
	for _, sub := range list {
		if err := c.walkSameInstance(sub, visiting, done); err != nil {
			return err
		}
	}
	return nil
}
