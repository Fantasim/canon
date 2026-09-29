package rules

// dot joins a variant and its case in the scope E5003 names.
const dot = "."

// fmtNoBag names the package the Runner has no bag for.
const fmtNoBag = "%w: %q"

// The forms of a path segment: none (a pair half keeps its pair's path), a field, a plain list
// index, a keyed list key, a map key, a table entry.
const (
	segSame segForm = iota
	segField
	segIndex
	segKey
	segMapKey
	segEntry
)

// memoEntries bounds the entries a Memo keeps: past it, it forgets them all (as eval's memo does).
const memoEntries = 1 << 20
