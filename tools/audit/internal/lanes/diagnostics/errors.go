package diagnostics

import "errors"

var (
	errCatalogue = errors.New("read the diagnostic catalogue")
	errNoCodes   = errors.New("the diagnostic catalogue holds no codes table")
)
