package main

import "errors"

var (
	errArgs     = errors.New("unexpected arguments")
	errNoOut    = errors.New("-out is required")
	errOutDirty = errors.New("-out must not already exist and hold files")
	errBadN     = errors.New("-n must be a positive number of entries")
	errWrite    = errors.New("cannot write the benchmark project")
)
