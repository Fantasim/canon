package build

import "errors"

// ErrUnknownLayer is a layer name no loaded package has a file for (API.md O4).
var ErrUnknownLayer = errors.New("unknown layer")
