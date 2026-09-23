package rules

import "errors"

// ErrNoBag is a package the Runner was given no bag for.
var ErrNoBag = errors.New("no bag for the package")
