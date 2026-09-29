package safego

import (
	"errors"
	"fmt"
)

// ErrPanic is a panic recovered on a goroutine Go started.
var ErrPanic = errors.New("panic on a background goroutine")

// PanicError is a recovered panic: its value and the stack of the goroutine that panicked.
type PanicError struct {
	Value any
	Stack string
}

func (e *PanicError) Error() string { return fmt.Sprintf(fmtPanic, ErrPanic, e.Value) }

func (e *PanicError) Unwrap() error { return ErrPanic }
