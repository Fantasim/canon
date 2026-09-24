// Package canon is the public Go API of the Canon compiler: the library behind the canon
// command, the language server and the studio.
//
// The package is the frozen contract, and spec/API.md its normative description. A body is a
// stub until the milestone that implements it: a stub with an error result returns an
// *InternalError, one without panics. Opening a project, its packages, its revision, Check and
// Build are implemented, as are the helpers the spec fixes entirely (value and op constructors,
// error texts, Version, the JSON and text forms of findings).
package canon
