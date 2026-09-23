package main

import "errors"

var (
	errArgs       = errors.New("unexpected arguments")
	errRuntimeDir = errors.New("not a directory")
)
