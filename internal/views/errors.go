package views

import "errors"

// ErrNoPackage is a view model asked of a package the checked program does not hold.
var ErrNoPackage = errors.New("views: no such package")
