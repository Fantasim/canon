// Sets each argument on both CANON_CHAIN_* variables, runs the generated Go demo.chain
// LoadInputs and prints one line per argument: the values read, then every failure line joined
// by " | " (CODEGEN.md §5.12, §7.7). The Go twin of chain_main.cpp (TestInputPatternChainParity).
package main

import (
	"fmt"
	"os"
	"strings"

	chain "example.com/parity/chain"
)

// show is an optional input's value, or none.
func show(v string, ok bool) string {
	if !ok {
		return "none"
	}
	return v
}

func main() {
	for _, text := range os.Args[1:] {
		os.Setenv("CANON_CHAIN_CODE", text)
		os.Setenv("CANON_CHAIN_TAG", text)
		failures := ""
		if err := chain.LoadInputs(); err != nil {
			failures = strings.ReplaceAll(err.Error(), "\n", " | ")
		}
		cfg := &chain.Config{}
		code, codeOK := cfg.Code()
		tag, tagOK := cfg.Tag()
		fmt.Printf("code=%s tag=%s err=%s\n", show(code, codeOK), show(tag, tagOK), failures)
	}
}
