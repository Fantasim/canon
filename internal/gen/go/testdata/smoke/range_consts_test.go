package rangeprobe

import (
	"testing"
	"time"
)

// CODEGEN.md §5.15, DECISIONS 400: an Int bound is an untyped constant (an array length, compared with len and a count), a Float bound a typed float64, a Duration bound a time.Duration.
var (
	_ [Probe_TagsLenMax]int
	_ [Probe_TagsElemMax]int
	_ [Probe_SLenMax]byte
)

func TestBoundConstants(t *testing.T) {
	tags := []int8{1, 2}
	if len(tags) > Probe_TagsLenMax || int64(len(tags)) > Probe_TagsLenMax || Probe_TagsLenMax != 3 {
		t.Errorf("tags: %d", Probe_TagsLenMax)
	}
	f32 := float32(0.1)
	if float64(f32) <= Probe_F32Max {
		t.Errorf("float32(0.1) must exceed the binary64 bound %v in binary64", Probe_F32Max)
	}
	var top float64 = Probe_F32Max
	if top != 0.1 || Probe_FMin != 0 || Probe_FEnd != 1 {
		t.Errorf("f32 %v, f %v..%v", top, Probe_FMin, Probe_FEnd)
	}
	var lo, hi time.Duration = Probe_DMin, Probe_DMax
	if lo != time.Second || hi != time.Hour {
		t.Errorf("d: %v..=%v", lo, hi)
	}
	if Probe_NMin != 1 || Probe_NMax != 120 || Probe_I8Max != 300 || Probe_U8Max != 255 || Probe_ExEnd != 5 {
		t.Error("n, i8, u8 or ex")
	}
	if Probe_MKeyLenMin != 1 || Probe_MKeyLenMax != 3 || Probe_MValueMax != 9 || Probe_MiKeyMax != 100 || Probe_OptsElemMax != 5 {
		t.Error("m, mi or opts")
	}
}
