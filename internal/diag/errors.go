package diag

import "errors"

var (
	// ErrWrite is a failure to write rendered findings.
	ErrWrite = errors.New("diag: cannot write findings")
	// ErrJSONString is a JSON string literal that RFC 8259 or WIRE.md §3.2 refuses.
	ErrJSONString = errors.New("invalid JSON string")

	errNoQuote      = errors.New("no opening quote")
	errUnterminated = errors.New("unterminated string")
	errControl      = errors.New("unescaped control character")
	errUTF8         = errors.New("invalid UTF-8")
	errEscape       = errors.New("invalid escape")
	errSurrogate    = errors.New("unpaired surrogate")
)
