// Package jsonschema validates a JSON document against a JSON Schema 2020-12 schema, using only
// the keyword subset spec/viewmodel.schema.json needs (VIEWMODEL.md's V2), so a view-model
// golden can be checked without a third-party dependency. Compile rejects any other keyword or a
// non-local $ref, so the validator can never silently skip a rule. Numbers are compared exactly
// (math/big), never as float64.
package jsonschema
