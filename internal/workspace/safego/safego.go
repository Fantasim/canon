package safego

import "runtime/debug"

// Go runs fn on a new goroutine, then done on that goroutine with fn's error, or with a
// *PanicError when fn panicked.
func Go(fn func() error, done func(error)) {
	go func() {
		done(Run(fn))
	}()
}

// Run is fn's error, or its panic as a *PanicError, on the calling goroutine.
func Run(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &PanicError{Value: r, Stack: string(debug.Stack())}
		}
	}()
	return fn()
}
