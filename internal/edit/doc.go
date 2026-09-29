// Package edit is the edit engine: value paths, operations, editability, cascades, minimal
// writes, file placement and the journal (spec/API.md).
//
// A Typer types an operation's value into a value.Value whose records hold only the fields
// the value writes: an unwritten field is nil with Set false, whatever its default, and applying
// the edit gives it its default (API.md V3). Nothing else in a typed value is nil.
package edit
