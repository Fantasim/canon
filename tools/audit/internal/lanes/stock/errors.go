package stock

import "errors"

var (
	// errNoToolchain means ctx.Toolchain is empty: nothing pins the tool versions.
	errNoToolchain = errors.New("stock: no toolchain configured")
	// errEmptyToolPath means `go tool -n` resolved to nothing.
	errEmptyToolPath = errors.New("stock: tool path did not resolve")
	// errConfig means golangci.yml could not be rendered with the thresholds.
	errConfig = errors.New("stock: render golangci config")
	// errOutOfSet means an issue's file is not one of this run's root-relative targets.
	errOutOfSet = errors.New("stock: golangci-lint reported a file outside the audited set")
	// errCacheBase means the lane was given no absolute directory to keep its own cache under.
	errCacheBase = errors.New("stock: golangci-lint cache base is not an absolute directory")
)
