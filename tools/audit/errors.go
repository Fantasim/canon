package main

import "errors"

var (
	errUnknownCommand = errors.New("unknown command")
	errUnknownRule    = errors.New("unknown rule")
)
