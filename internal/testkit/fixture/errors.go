package fixture

import "errors"

var (
	errWalk  = errors.New("walk the example tree")
	errRead  = errors.New("read an example file")
	errWrite = errors.New("write a fixture file")
	errCopy  = errors.New("copy an example tree")
)
