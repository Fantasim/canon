package format

import "errors"

// ErrSyntax is returned for a file with a syntax error: it is left unchanged (FORMATTER.md §1).
var ErrSyntax = errors.New("format: the file has syntax errors")
