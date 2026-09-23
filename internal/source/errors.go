package source

import "errors"

// ErrFileTooLarge is returned for content whose offsets do not fit a Pos.
var ErrFileTooLarge = errors.New("file too large for byte positions")
