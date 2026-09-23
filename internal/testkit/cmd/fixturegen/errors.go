package main

import "errors"

var (
	errArgs       = errors.New("unexpected arguments")
	errNoRealData = errors.New("no real data: copy the real roots into this directory first")
	errManifest   = errors.New("invalid fixtures.tsv row")
	errExtract    = errors.New("cannot extract")
	errBudget     = errors.New("the fixture tree would exceed its size budget")
)
