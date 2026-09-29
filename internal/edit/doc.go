// Package edit is the edit engine: value paths, operations, editability, cascades, minimal
// writes, file placement and the journal (spec/API.md).
//
// A Typer types an operation's value into a value.Value whose records hold only the fields
// the value writes: an unwritten field is nil with Set false, whatever its default, and applying
// the edit gives it its default (API.md V3). Nothing else in a typed value is nil. Apply computes
// an edit in memory against a Snapshot, each operation against the state the previous ones left,
// re-printing only the items whose value changed; it takes no lock, writes and publishes nothing:
// its Plan is for the caller's transaction to check and commit.
// Journals are untrusted input; recovery and its threat model: meta/decisions/0010-edit-journal-recovery.md.
package edit
