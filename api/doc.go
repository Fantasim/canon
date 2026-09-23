// Package canon is the public Go API of the Canon compiler: the library behind the canon
// command, the language server and the studio.
//
// The package is the frozen contract; spec/API.md is its normative description, with every
// rule cited here (O1, S5, E14, …). Bodies are stubs until milestone M4 of
// spec/IMPLEMENTATION-PLAN.md: a stub with an error result returns an *InternalError, one
// without panics. Helpers whose behaviour the spec fixes entirely (value and op constructors,
// error texts, Version) are implemented already.
package canon
