package safego_test

import (
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

func Example() {
	results := make(chan error)
	safego.Go(func() error { panic("bug") }, func(err error) { results <- err })
	err := <-results
	var pe *safego.PanicError
	fmt.Println(errors.Is(err, safego.ErrPanic), errors.As(err, &pe) && pe.Value == "bug" && pe.Stack != "")
	safego.Go(func() error { return nil }, func(err error) { results <- err })
	fmt.Println(<-results)
	// Output:
	// true true
	// <nil>
}

func ExampleRun() {
	err := safego.Run(func() error { panic("bug") })
	fmt.Println(errors.Is(err, safego.ErrPanic), safego.Run(func() error { return nil }))
	// Output:
	// true <nil>
}
