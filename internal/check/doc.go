// Package check resolves names and type-checks declarations, bodies, views, translations and
// layers, recording every conclusion for the later phases (spec/TYPES.md).
//
// CheckSession keeps a Session whose Recheck re-checks only the files an edit changed. A changed
// file keeps its header and declarations: each is the checked node itself, which a parse shares
// when the edit left it before it, kept with its types, or an `entry` or annotated `let` changed
// only in its value and doc comment, keys kept, which is checked again. Every object of a changed
// file is renewed with its type. The result equals Check's; Recheck refuses anything else, a
// parse finding, a kept finding reading a changed file, a stale session and a cancelled context.
package check
