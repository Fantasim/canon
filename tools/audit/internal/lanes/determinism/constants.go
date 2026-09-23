package determinism

import "github.com/fantasim/canonlang/tools/audit/internal/rules"

const laneName = "determinism"

const ruleMapRange = rules.IDMapRange

var ruleIDs = []string{ruleMapRange}

// outputDirs are the output packages and those under them (IMPLEMENTATION-PLAN.md §7.5).
var outputDirs = []string{
	"api", "internal/build", "internal/diag", "internal/edit", "internal/format", "internal/gen",
	"internal/i18n", "internal/ir", "internal/jsonsrc", "internal/lock", "internal/views", "internal/wire",
}

// The marker of an order-free loop, alone at the start of a line comment, then its reason.
const marker = "//canon:unordered"

// mapIterators are the functions of package maps whose sequences follow map order.
var mapIterators = []string{"All", "Keys", "Values"}

const (
	pkgMaps = "maps"
	pathSep = "/"
)

// What the rule reports, as finding details and messages.
const (
	detailRange    = "map range"
	detailIterator = "maps iterator"
	detailReason   = "marker without reason"
	detailStale    = "stale marker"

	msgRange    = "range over %s in an output package: its order is random; range over sorted keys, or mark an order-free loop " + marker + " <reason>"
	msgIterator = "range over maps.%s in an output package: its order is random; use slices.Sorted, or mark an order-free loop " + marker + " <reason>"
	msgReason   = marker + " without a reason: say why the order cannot reach the output"
	msgStale    = marker + " marks no range over a map on its line or the next"
)
