package main

import "errors"

var (
	errArgs      = errors.New("unexpected arguments")
	errTrailing  = errors.New("data after the schema")
	errDuplicate = errors.New("member named twice in one object")
	errKeyword   = errors.New("unknown schema keyword")
	errSchema    = errors.New("unsupported schema construct")
	errRef       = errors.New("unresolved $ref")
	errRequired  = errors.New("required names an undeclared member")
	errConflict  = errors.New("branches disagree on a member's type")
	errOrder     = errors.New("branches disagree on member order")
	errName      = errors.New("type or field name taken twice, or not an identifier")
	errOwner     = errors.New("definition is a branch of two unions")
	errUnreached = errors.New("object definition unreachable from the root")
	errStaleName = errors.New("name override matches no schema object")
	errObject    = errors.New("object schema not closed: type object and additionalProperties false required")
	errAlias     = errors.New("definition that is only a $ref")
	errCycle     = errors.New("definition reaches itself through $ref, oneOf or items without an object between")
)
