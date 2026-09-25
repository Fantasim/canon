package config_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

// Example shows the variable name RequireCxx reads.
func Example() {
	fmt.Println(config.RequireCxxEnv)
	// Output: CANON_REQUIRE_CXX
}
