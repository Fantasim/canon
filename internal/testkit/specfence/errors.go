package specfence

import "errors"

// ErrNoHeading is returned when the document does not contain the heading.
var ErrNoHeading = errors.New("specfence: the heading is not in the document")

// ErrNoBlock is returned when no block of the language follows the heading.
var ErrNoBlock = errors.New("specfence: no block of the language after the heading")

// ErrUnclosed is returned when the block after the heading is not closed.
var ErrUnclosed = errors.New("specfence: the block after the heading is not closed")
