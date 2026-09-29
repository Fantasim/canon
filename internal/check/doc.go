// Package check resolves names and type-checks declarations, bodies, views, translations and
// layers, recording every conclusion for the later phases (spec/TYPES.md).
//
// CheckSession keeps a Session whose Recheck re-checks only the files an edit changed. Each
// changed file holds only `entry` declarations, writes no type and differs from the file of its
// path only in its entries' values and doc comments. Recheck's result equals Check's over the
// new files, findings included, and keeps the objects and types of the unchanged files. It
// refuses anything else, a kept finding that reads a changed file, a session that is not its
// lineage's latest and a cancelled context.
package check
