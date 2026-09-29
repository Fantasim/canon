package typedef

import "errors"

// ErrNoTypeFunc is a type function the drivers' program does not declare, though built from the
// same sources as the model's: a compiler bug.
var ErrNoTypeFunc = errors.New("typedef: the drivers' program lacks a type function")
