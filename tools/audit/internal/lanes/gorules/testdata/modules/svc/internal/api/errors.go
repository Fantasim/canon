package api

import "errors"

var ErrSentinel = errors.New("sentinel")

const errCode = "E-code"

var _ = errCode
