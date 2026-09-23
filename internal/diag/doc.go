// Package diag is the registry of Canon's diagnostics and the builders of findings.
//
// spec/ERRORS.md is the single source of every code: codes.go is generated from it by
// cmd/diaggen and never edited by hand. A finding is built only by its code's typed
// constructor, diag.E3501.At(span, key, coll), whose arguments follow ERRORS.md.
package diag
