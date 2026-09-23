package golden

import "errors"

var (
	errNoCases = errors.New("no golden case matches")
	errNoWant  = errors.New("no want file")
	errDiffers = errors.New("output differs from the golden")
)
