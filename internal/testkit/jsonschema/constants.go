package jsonschema

// keyword is one member of the vocabulary; its one spelling lives in keywordOf, never a
// second string constant (api/vm/internal/vmgen, package main, reads the same vocabulary).
type keyword int

const (
	kwSchema keyword = iota
	kwID
	kwRef
	kwDefs
	kwAdditionalProperties
	kwAllOf
	kwConst
	kwDependentRequired
	kwDescription
	kwEnum
	kwIf
	kwItems
	kwMaxItems
	kwMaximum
	kwMinItems
	kwMinLength
	kwMinimum
	kwOneOf
	kwPattern
	kwProperties
	kwPropertyNames
	kwRequired
	kwThen
	kwTitle
	kwType
	kwUniqueItems
	kwCount // one past the last keyword: keywordNames' length, never a keyword itself
)

// keywordOf is the one source of truth: every spelling, as a map key (magic-string's own
// exemption), to the keyword it names. compileNode's allow-list is keywordOf's key set.
var keywordOf = map[string]keyword{
	"$schema": kwSchema, "$id": kwID, "$ref": kwRef, "$defs": kwDefs,
	"additionalProperties": kwAdditionalProperties, "allOf": kwAllOf, "const": kwConst,
	"dependentRequired": kwDependentRequired, "description": kwDescription, "enum": kwEnum,
	"if": kwIf, "items": kwItems, "maxItems": kwMaxItems, "maximum": kwMaximum,
	"minItems": kwMinItems, "minLength": kwMinLength, "minimum": kwMinimum, "oneOf": kwOneOf,
	"pattern": kwPattern, "properties": kwProperties, "propertyNames": kwPropertyNames,
	"required": kwRequired, "then": kwThen, "title": kwTitle, "type": kwType,
	"uniqueItems": kwUniqueItems,
}

// keywordNames is keywordOf's reverse, built from it (not hand-written): a keyword's spelling.
var keywordNames = func() (a [kwCount]string) {
	for s, k := range keywordOf { //canon:unordered exactly one key maps to any k
		a[k] = s
	}
	return a
}()

// keywordName is k's one spelling.
func keywordName(k keyword) string {
	return keywordNames[k]
}

// JSON instance kinds (2020-12 §4.2.1), plus "integer" as a type value only.
const (
	kindNull    = "null"
	kindBoolean = "boolean"
	kindObject  = "object"
	kindArray   = "array"
	kindNumber  = "number"
	kindString  = "string"
	kindInteger = "integer"
)

// refPrefix is the only local $ref shape Compile accepts: "#/$defs/<name>".
var refPrefix = "#/" + keywordName(kwDefs) + "/"

// rootPointer is the instance and schema JSON pointer of the document root.
const rootPointer = ""

// draft202012URI is the only value the root's "$schema", when present, may hold.
const draft202012URI = "https://json-schema.org/draft/2020-12/schema"

// schemaLabel and documentLabel name Compile and Validate's bytes, for jsonsrc's diagnostics.
const (
	schemaLabel   = "schema"
	documentLabel = "document"
)

// msgNotRepresentable is validateNumber's violation for a number too large to compare exactly.
const msgNotRepresentable = "number not exactly representable"

// jsonTypes is every value "type" may name, 2020-12's seven JSON kinds plus "integer".
var jsonTypes = map[string]bool{
	kindNull: true, kindBoolean: true, kindObject: true, kindArray: true,
	kindNumber: true, kindString: true, kindInteger: true,
}

// rootOnlyKeywords are refused anywhere but the schema root (compileNode).
var rootOnlyKeywords = map[keyword]bool{kwSchema: true, kwID: true, kwDefs: true}
