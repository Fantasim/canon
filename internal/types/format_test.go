package types_test

import (
	"math"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// STDLIB.md §9.2: ECMAScript Number::toString over the shortest round-tripping digits.
func TestFloatText(t *testing.T) {
	cases := []struct {
		x    float64
		bits int
		want string
	}{
		{1.0, 64, "1"},
		{0.15, 64, "0.15"},
		{2.5e6, 64, "2500000"},
		{1e21, 64, "1e+21"},
		{1e-7, 64, "1e-7"},
		{123e-20, 64, "1.23e-18"},
		{1e-6, 64, "0.000001"},
		{-1.5, 64, "-1.5"},
		{math.Copysign(0, -1), 64, "0"},
		{1.2345678901234568e20, 64, "123456789012345680000"},
		{float64(float32(0.1)), 32, "0.1"},
		{float64(float32(0.1)), 64, "0.10000000149011612"},
		{-2.5e-300, 64, "-2.5e-300"},
		{1e100, 64, "1e+100"},
	}
	for _, c := range cases {
		if got := types.FloatText(c.x, c.bits); got != c.want {
			t.Errorf("FloatText(%v, %d) = %q, want %q", c.x, c.bits, got, c.want)
		}
	}
}

// STDLIB.md §9.3: the canonical duration literal.
func TestDurationText(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{90_000, "1m30s"},
		{172_800_000, "2d"},
		{5_400_000, "1h30m"},
		{1500, "1s500ms"},
		{0, "0s"},
		{-250, "-250ms"},
		{math.MinInt64, "-106751991167d7h12m55s808ms"},
	}
	for _, c := range cases {
		if got := types.DurationText(c.ms); got != c.want {
			t.Errorf("DurationText(%d) = %q, want %q", c.ms, got, c.want)
		}
	}
}

// STDLIB.md §9.4: nested strings are quoted with Canon escapes, UTF-8 kept.
func TestQuoteString(t *testing.T) {
	got := types.QuoteString("a\"b\\c\n\t\r\x01\x7fé{}")
	if want := `"a\"b\\c\n\t\r\u{1}\u{7F}é{}"`; got != want {
		t.Errorf("QuoteString = %s, want %s", got, want)
	}
}
