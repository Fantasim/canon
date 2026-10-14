package syntax_test

import (
	"os"
	"testing"
)

// largeExample is a large .canon file of examples/, the parse the benchmark measures.
const largeExample = examplesDir + "/telemetry/telemetry.canon"

// BenchmarkParse measures the bytes parsing a large example allocates (IMPLEMENTATION-PLAN §4.1).
func BenchmarkParse(b *testing.B) {
	data, err := os.ReadFile(largeExample)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		parseText(b, "telemetry/telemetry.canon", data)
	}
}
