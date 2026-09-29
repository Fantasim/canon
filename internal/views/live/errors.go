package live

import "errors"

// ErrNoLanguages is a Target.Lang given without Input.Languages: the source language cannot be
// told apart from a language the project lacks (API.md V8).
var ErrNoLanguages = errors.New("live: a language needs the project's languages")
