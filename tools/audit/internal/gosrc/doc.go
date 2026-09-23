// Package gosrc parses a repo's Go files once for every lane, maps a line to its enclosing
// declaration, and loads type information lazily for the lanes that need it.
package gosrc
