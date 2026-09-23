package gorules

import (
	"errors"

	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

var (
	errImports = errors.New("read " + repo.ImportsFile)
	errTyped   = errors.New("type-checked load has errors")
)
