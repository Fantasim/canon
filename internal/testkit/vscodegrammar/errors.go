package vscodegrammar

import "errors"

var (
	errNestedCapture   = errors.New("overlapping or nested captures are outside the supported subset")
	errMisplacedAnchor = errors.New("^ must be the first element of a pattern")
	errUnknownRule     = errors.New("grammar includes an unknown repository rule")
)
