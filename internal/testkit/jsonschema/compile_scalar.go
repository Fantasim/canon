package jsonschema

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
)

// compileType compiles type: a string, or a non-empty array of strings with no repeat.
func (c *compiler) compileType(n *node, raw map[string]any, path string) error {
	rawType, ok := raw[keywordName(kwType)]
	if !ok {
		return nil
	}
	kwPath := appendToken(path, keywordName(kwType))
	names, err := decodeTypeNames(rawType, kwPath)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%s: %w", kwPath, errEmptyArray)
	}
	for _, name := range names {
		if !jsonTypes[name] {
			return fmt.Errorf("%s: %w", kwPath, errBadType)
		}
	}
	if err := checkNoDuplicateStrings(names, kwPath); err != nil {
		return err
	}
	n.types = names
	return nil
}

// decodeTypeNames is type's value, a string or an array of strings.
func decodeTypeNames(rawType any, kwPath string) ([]string, error) {
	switch t := rawType.(type) {
	case string:
		return []string{t}, nil
	case []any:
		return stringSlice(t, kwPath)
	default:
		return nil, fmt.Errorf("%s: %w", kwPath, errBadType)
	}
}

// checkNoDuplicateStrings reports the first value list repeats (2020-12's own uniqueness
// rules for type and required).
func checkNoDuplicateStrings(list []string, kwPath string) error {
	seen := make(map[string]bool, len(list))
	for _, s := range list {
		if seen[s] {
			return fmt.Errorf("%s: %w", kwPath, errDuplicateValue)
		}
		seen[s] = true
	}
	return nil
}

// compileConstEnum compiles const, any JSON value, and enum, an array of them.
func (c *compiler) compileConstEnum(n *node, raw map[string]any, path string) error {
	if v, ok := raw[keywordName(kwConst)]; ok {
		n.hasConst = true
		n.constVal = v
	}
	rawEnum, ok := raw[keywordName(kwEnum)]
	if !ok {
		return nil
	}
	list, ok := rawEnum.([]any)
	if !ok {
		return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwEnum)), errBadArray)
	}
	n.enumVals = list
	return nil
}

// compileStringKeywords compiles minLength and pattern.
func (c *compiler) compileStringKeywords(n *node, raw map[string]any, path string) error {
	if rawMin, ok := raw[keywordName(kwMinLength)]; ok {
		count, err := jsonInt(rawMin)
		if err != nil {
			return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwMinLength)), err)
		}
		n.minLength = &count
	}
	rawPattern, ok := raw[keywordName(kwPattern)]
	if !ok {
		return nil
	}
	s, ok := rawPattern.(string)
	if !ok {
		return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwPattern)), errBadString)
	}
	re, err := regexp.Compile(s)
	if err != nil {
		return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwPattern)), errBadPattern)
	}
	n.pattern = re
	return nil
}

// compileNumberKeywords compiles minimum and maximum, parsed exactly (never float64).
func (c *compiler) compileNumberKeywords(n *node, raw map[string]any, path string) error {
	if rawMin, ok := raw[keywordName(kwMinimum)]; ok {
		r, err := jsonRat(rawMin)
		if err != nil {
			return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwMinimum)), err)
		}
		n.minimum = r
	}
	if rawMax, ok := raw[keywordName(kwMaximum)]; ok {
		r, err := jsonRat(rawMax)
		if err != nil {
			return fmt.Errorf("%s: %w", appendToken(path, keywordName(kwMaximum)), err)
		}
		n.maximum = r
	}
	return nil
}

// compileAnnotations checks title, description and $id are strings, when present; none is
// stored, since none constrains an instance.
func (c *compiler) compileAnnotations(_ *node, raw map[string]any, path string) error {
	if err := checkOptionalString(raw, kwTitle, path); err != nil {
		return err
	}
	if err := checkOptionalString(raw, kwID, path); err != nil {
		return err
	}
	return checkOptionalString(raw, kwDescription, path)
}

// checkOptionalString requires kw's value to be a string, when raw holds one.
func checkOptionalString(raw map[string]any, kw keyword, path string) error {
	v, ok := raw[keywordName(kw)]
	if !ok {
		return nil
	}
	if _, ok := v.(string); !ok {
		return fmt.Errorf("%s: %w", appendToken(path, keywordName(kw)), errBadString)
	}
	return nil
}

// jsonRat converts a decodeJSON number to an exact rational.
func jsonRat(v any) (*big.Rat, error) {
	n, ok := v.(json.Number)
	if !ok {
		return nil, errBadNumber
	}
	r, ok := toRat(n)
	if !ok {
		return nil, errBadNumber
	}
	return r, nil
}

// jsonInt converts a decodeJSON number to a non-negative int, rejecting a fractional value
// and one too large for int64 to hold exactly (big.Int.Int64 truncates silently otherwise).
func jsonInt(v any) (int, error) {
	r, err := jsonRat(v)
	if err != nil {
		return 0, err
	}
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, errBadCount
	}
	i64 := r.Num().Int64()
	if i64 < 0 {
		return 0, errBadCount
	}
	return int(i64), nil
}
