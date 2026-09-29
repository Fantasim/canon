package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
)

// BadCache keeps per source file whether its tree holds a Bad node; nil keeps nothing (IMPLEMENTATION-PLAN §7.6).
type BadCache struct {
	bad check.FileCache[bool]
}

// files is c's per-file cache, nil for a nil c.
func (c *BadCache) files() *check.FileCache[bool] {
	if c == nil {
		return nil
	}
	return &c.bad
}
