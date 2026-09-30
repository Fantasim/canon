// Package project reads project.canon: its schema, the named roots, path normalization, and
// the discovery of packages and their files (SPEC.md).
//
// A Reader given a Reuse returns the kept tree of an unchanged file; the new parse of a changed
// one shares the old parse's header and the declarations wholly before its first changed byte.
package project
