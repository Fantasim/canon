package must_test

import (
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/must"
)

// Example shows the call a test makes where a failure is not the case under test: a nil error
// returns and the test goes on.
func Example() {
	must.NoError(new(testing.T), nil)
	fmt.Println("went on")
	// Output: went on
}
