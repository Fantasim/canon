package golden

import "errors"

var (
	errNoCases    = errors.New("no golden case matches")
	errNoWant     = errors.New("no expected file")
	errNoExpected = errors.New("empty expected file name")
	errDiffers    = errors.New("output differs from the golden")
)
