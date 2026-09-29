package types

import "slices"

// DurationUnits is every wire unit of a Duration, largest first (WIRE.md §5.1).
func DurationUnits() []Unit { return slices.Clone(durationParts[:]) }
