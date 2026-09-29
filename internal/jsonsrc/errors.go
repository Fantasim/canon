package jsonsrc

import (
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/source"
)

var (
	// ErrSyntax is a source that is not JSON; Parse has reported its one E7109.
	ErrSyntax = errors.New("jsonsrc: syntax error")
	// ErrDuplicateKey is a source with a repeated key; Parse has reported an E7104 for each.
	ErrDuplicateKey = errors.New("jsonsrc: duplicate key")
	// ErrEncoding is a source whose text is not UTF-8.
	ErrEncoding = errors.New("jsonsrc: not valid UTF-8")
	// ErrEdit is an Edit that does not apply to its source.
	ErrEdit = errors.New("jsonsrc: the edit does not apply")
)

// EncodingError locates load's E7105 (DECISIONS 163); Surrogate is a lone \u surrogate, else 0.
type EncodingError struct {
	Span      source.Span
	Surrogate rune
}

func (e *EncodingError) Error() string {
	return fmt.Sprintf("%v at offset %d of the normalized content", ErrEncoding, e.Span.Start)
}

func (e *EncodingError) Unwrap() error { return ErrEncoding }
