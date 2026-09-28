package jsonschema

import "errors"

// Compile-time misuse: sentinels, wrapped with %w (go.md §3).
var (
	errUnknownKeyword  = errors.New("unsupported schema keyword")
	errNonLocalRef     = errors.New("$ref is not a local \"#/$defs/<name>\" reference")
	errUndefinedRef    = errors.New("$ref names no entry of $defs")
	errRefCycle        = errors.New("$ref forms a cycle that never reaches a different part of the instance")
	errNotASchema      = errors.New("not a schema: want a JSON object or a boolean")
	errBadType         = errors.New("type is not one of null, boolean, object, array, number, string, integer")
	errBadPattern      = errors.New("pattern does not compile as a regexp")
	errBadNumber       = errors.New("not a JSON number")
	errBadCount        = errors.New("not a non-negative integer")
	errBadTopLevel     = errors.New("schema root is not a JSON object")
	errBadStringList   = errors.New("not an array of strings")
	errBadArray        = errors.New("not a JSON array")
	errBadObject       = errors.New("not a JSON object")
	errBadString       = errors.New("not a JSON string")
	errBadBool         = errors.New("not a JSON boolean")
	errEmptyArray      = errors.New("must be a non-empty array")
	errDuplicateValue  = errors.New("contains the same value twice")
	errBadSchemaURI    = errors.New("$schema does not name draft 2020-12")
	errRootOnlyKeyword = errors.New("may only appear on the schema root")
)
