package config_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/tsc/config"
)

func Example() {
	fmt.Println(config.RequireTSEnv)
	// Output: CANON_REQUIRE_TS
}
