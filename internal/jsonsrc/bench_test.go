package jsonsrc_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// synthetic is a source of n item rows shaped like propItem.json, about 300 bytes each.
func synthetic(n int) string {
	var b strings.Builder
	b.WriteString("{\n  \"version\": 1,\n  \"items\": [\n")
	for i := range n {
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, `    {"dwID": "II_GEN_%06d", "szName": "IDS_PROPITEM_TXT_%06d", "dwItemLV": %d, `+
			`"fWeight": %d.25, "bPermanence": true, "tags": ["a\"b", "é\n", null], "legacy": {"x": -1e3}}`,
			i, i, i%120, i%7)
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// IMPLEMENTATION-PLAN.md §7.6: reading and printing a 7,000-row source (about 2 MB).
func BenchmarkParse(b *testing.B) {
	text := synthetic(7000)
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	for b.Loop() {
		mustParse(b, text)
	}
}

func BenchmarkFormat(b *testing.B) {
	root := mustParse(b, synthetic(7000)).root
	b.ReportAllocs()
	for b.Loop() {
		jsonsrc.Format(root)
	}
}
