package syntax

import "strings"

var reservedSegmentSet map[string]bool

func init() {
	reservedSegmentSet = map[string]bool{}
	for s := range strings.FieldsSeq(reservedSegments) {
		reservedSegmentSet[s] = true
	}
}

// IsReservedSegment reports a translation key's kind word or other reserved part (I18N.md K4).
func IsReservedSegment(s string) bool { return reservedSegmentSet[s] }

// Qualified is q's parts, dot-joined as written (I18N.md §3.2).
func Qualified(q *QualifiedName) string {
	parts := make([]string, len(q.Parts))
	for i, p := range q.Parts {
		parts[i] = p.Name
	}
	return strings.Join(parts, ".")
}
