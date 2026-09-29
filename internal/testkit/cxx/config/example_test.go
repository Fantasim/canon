package config_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

func Example() {
	fmt.Println(config.RequireCxxEnv)
	// Output: CANON_REQUIRE_CXX
}

func Example_nlohmannInclude() {
	fmt.Println(config.NlohmannIncludeEnv)
	// Output: CANON_NLOHMANN_INCLUDE
}

func Example_requireMSVC() {
	fmt.Println(config.RequireMSVCEnv)
	// Output: CANON_REQUIRE_MSVC
}
