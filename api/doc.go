// Package canon is the public Go API of the Canon compiler: the library behind the canon
// command, the language server and the studio.
//
// The package is the frozen contract, and spec/API.md its normative description. A body is a
// stub until the milestone that implements it: a stub with an error result returns an
// *InternalError, one without panics, and Open and Revision land in one change, so that no
// Example reaches a panicking stub. Helpers whose behaviour the spec fixes entirely (value and
// op constructors, error texts, Version, the JSON and text forms of findings) are implemented
// already.
package canon
