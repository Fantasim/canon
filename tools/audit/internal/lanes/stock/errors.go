package stock

import "errors"

var (
	// errNoToolchain means ctx.Toolchain is empty: nothing pins the tool versions.
	errNoToolchain = errors.New("stock: no toolchain configured")
	// errEmptyToolPath means `go tool -n` resolved to nothing.
	errEmptyToolPath = errors.New("stock: tool path did not resolve")
)
