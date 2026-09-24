package lock

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// LayerAmendment is an amendment a layer may not make (LOCK.md §6.1), located at Span.
type LayerAmendment struct {
	Layer string
	Table string // the stable table an entry would be added to; "" for a field
	Field string
	Span  source.Span
}

// ReportLayer reports a forbidden layer amendment to bag (E6004, LOCK.md §6.1).
func ReportLayer(a LayerAmendment, bag *diag.Bag) {
	if a.Table != "" {
		diag.E6004.AtEntry(a.Span, a.Layer, a.Table).Report(bag)
		return
	}
	diag.E6004.AtField(a.Span, a.Layer, a.Field).Report(bag)
}
