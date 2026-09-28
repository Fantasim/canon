package jsonschema

import (
	"math/big"
	"regexp"
)

// Schema is a compiled JSON Schema 2020-12 document, restricted to the keyword subset doc.go
// names. It is immutable once Compile returns it.
type Schema struct {
	root *node
	defs map[string]*node
}

// node is one compiled schema (a JSON object using only the allowed keywords, or a boolean
// schema). A nil pointer field means the keyword was absent; validate* functions treat absence
// as "no constraint".
type node struct {
	boolSchema *bool
	ref        string // $defs name after refPrefix
	types      []string
	hasConst   bool
	constVal   any
	enumVals   []any

	minLength *int
	pattern   *regexp.Regexp
	minimum   *big.Rat
	maximum   *big.Rat

	minItems    *int
	maxItems    *int
	uniqueItems bool
	items       *node

	properties              map[string]*node
	propertyNames           *node
	additionalProperties    *node
	hasAdditionalProperties bool
	required                []string
	dependentRequired       map[string][]string

	allOf []*node
	oneOf []*node
	ifs   *node
	then  *node
}
