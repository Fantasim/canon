package ir

import "github.com/fantasim/canonlang/internal/types"

// Target is the language or format of an emit.
type Target uint8

// Mode is the mode of a code emit (CODEGEN.md §2.2).
type Mode uint8

type Access uint8

// NameOptions is a @go(name:), @cpp(name:) or @ts(name:) override; "" keeps the derived name.
type NameOptions struct {
	Name string
}

// CppOptions are the @cpp arguments of a record header (CODEGEN.md §7.8).
type CppOptions struct {
	Name   string
	Struct string
	Header string
	Access Access
}

// CppFieldOptions are @cpp(name:, field:, type:, unit:) on a field; "" keeps the default.
type CppFieldOptions struct {
	Name    string
	Member  string
	Type    string
	Unit    types.Unit
	HasUnit bool
}

// CppCaseOptions are @cpp(name:, value:) on a variant case.
type CppCaseOptions struct {
	Name     string
	Value    int64
	HasValue bool
}
