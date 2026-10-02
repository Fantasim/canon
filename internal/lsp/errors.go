package lsp

import "errors"

// ErrNoShutdown is Serve ending on exit or end of input before a shutdown request: the
// protocol's exit code 1.
var ErrNoShutdown = errors.New("lsp: exit before shutdown")

var (
	errHeader         = errors.New("lsp: invalid message header")
	errWrite          = errors.New("lsp: write")
	errTooLarge       = errors.New("lsp: message too large")
	errParse          = errors.New("parse error")
	errInvalidRequest = errors.New("invalid request")
	errMethodNotFound = errors.New("method not found")
	errInvalidParams  = errors.New("invalid params")
	errNotInitialized = errors.New("server not initialized")
	errFileSet        = errors.New("lsp: findings without a file set")
	errDisk           = errors.New("lsp: disk")
	errFind           = errors.New("lsp: finding the project")
	errNotFormatted   = errors.New("lsp: not formatted")
	errRefs           = errors.New("lsp: references")
	errNoFile         = errors.New("in no file")
)

// errorCodes maps the protocol's failures to their codes; anything else is an internal error.
var errorCodes = [...]struct {
	err  error
	code int
}{
	{errParse, codeParseError},
	{errInvalidRequest, codeInvalidRequest},
	{errTooLarge, codeInvalidRequest},
	{errMethodNotFound, codeMethodNotFound},
	{errInvalidParams, codeInvalidParams},
	{errNotInitialized, codeNotInitialized},
	{errNotFormatted, codeRequestFailed},
	{errRefs, codeRequestFailed},
}
