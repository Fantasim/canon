package jsongen

import "errors"

// What Generate refuses instead of reporting a finding (decision 126).
var (
	// ErrEmit is a nil package or emit, or an emit whose target is not json.
	ErrEmit = errors.New("jsongen: not a json emit")
	// ErrValues is a values list naming no public value of the package, or one twice.
	ErrValues = errors.New("jsongen: values do not select public values once each")
	// ErrFileMode is a file-mode emit whose file is not a .json name or holds other than one value.
	ErrFileMode = errors.New("jsongen: a file-mode emit writes exactly one value to a .json file")
	// ErrDataMode is a data-mode or @reload value this emit does not write as <value>.json.
	ErrDataMode = errors.New("jsongen: data-mode link broken")
	// ErrSchema is a Value.Schema other than the fingerprint of the file written for it.
	ErrSchema = errors.New("jsongen: Value.Schema differs from its file")
	// ErrFn is a stored export fn without its result, table or per-receiver instance.
	ErrFn = errors.New("jsongen: export fn without its stored results")
)
