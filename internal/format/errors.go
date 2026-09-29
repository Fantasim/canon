package format

import "errors"

// The errors of this package; Rewrite's are pinned by log-2026-09-29 M4 U1r.
var (
	ErrSyntax    = errors.New("format: the file has syntax errors")                            // left unchanged (FORMATTER.md §1)
	ErrLines     = errors.New("format: the node cannot be printed on one line")                // Flat
	ErrNode      = errors.New("format: the node is printed only with the node holding it")     // Node, Flat: an interpolation, a doc comment, a node of another file
	ErrPlace     = errors.New("format: a place cannot be negative")                            // Node
	ErrChange    = errors.New("format: the change does not apply")                             // Rewrite: to this tree
	ErrText      = errors.New("format: the new text does not parse in its place")              // Rewrite: or not as one node of its kind
	ErrLayout    = errors.New("format: the text holds a carriage return or a tab indentation") // Rewrite: format it whole first (API.md M9)
	ErrUnsettled = errors.New("format: no enclosing item makes the file a fixed point")        // Rewrite: a fault of the formatter (API.md M5)
)
