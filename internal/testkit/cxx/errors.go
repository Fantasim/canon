package cxx

import "errors"

// errOutputSet is Compile's error for a command whose Stdout or Stderr is already set.
var errOutputSet = errors.New("cxx: Stdout or Stderr already set")
