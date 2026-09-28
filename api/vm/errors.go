package vm

import "errors"

var (
	// ErrNumber is a Number whose text is not a JSON number or a decimal string (J10).
	ErrNumber = errors.New("vm: not a view-model number")
	// ErrScalar is a Scalar that is neither a string nor an integer.
	ErrScalar = errors.New("vm: not a string or an integer")
	// ErrTextRef is a TextRef that is neither a key nor a language-neutral text (J9).
	ErrTextRef = errors.New("vm: not a text reference")
)
