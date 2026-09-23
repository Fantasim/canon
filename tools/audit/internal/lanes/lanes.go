package lanes

import (
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/lanes/diagnostics"
	"github.com/fantasim/canonlang/tools/audit/internal/lanes/gorules"
	"github.com/fantasim/canonlang/tools/audit/internal/lanes/gostyle"
	"github.com/fantasim/canonlang/tools/audit/internal/lanes/integrity"
	"github.com/fantasim/canonlang/tools/audit/internal/lanes/project"
	"github.com/fantasim/canonlang/tools/audit/internal/lanes/stock"
)

func All() []lane.Lane {
	return []lane.Lane{gostyle.New(), gorules.New(), diagnostics.New(), stock.New(), project.New(), integrity.New()}
}
