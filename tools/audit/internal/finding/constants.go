package finding

import "regexp"

const (
	keySep    = "\t"
	maxDetail = 160
)

var (
	digits = regexp.MustCompile(`\d+`)
	spaces = regexp.MustCompile(`\s+`)
)
