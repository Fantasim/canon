package wire

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

func float64Text(t *testing.T, x float64, bits int) string {
	t.Helper()
	typ := types.FloatType
	if bits == float32Bits {
		typ = types.Float32Type
	}
	n, err := floatNode(&value.Float{V: x, T: typ})
	if err != nil {
		t.Fatalf("floatNode(%v): %v", x, err)
	}
	return string(n.raw)
}

// WIRE.md §7.2: the table of canonical numbers.
func TestNumbers(t *testing.T) {
	cases := []struct {
		x    float64
		bits int
		want string
	}{
		{0.1, 64, "0.1"}, {1.0, 64, "1"}, {math.Copysign(0, -1), 64, "0"}, {-2.5, 64, "-2.5"},
		{100.0, 64, "100"}, {123.456, 64, "123.456"}, {1.0 / 3, 64, "0.3333333333333333"},
		{0.000001, 64, "0.000001"}, {1e-7, 64, "1e-7"}, {1.5e-7, 64, "1.5e-7"},
		{1e20, 64, "100000000000000000000"}, {1e21, 64, "1e+21"}, {1e300, 64, "1e+300"},
		{1.7976931348623157e308, 64, "1.7976931348623157e+308"}, {5e-324, 64, "5e-324"},
		{9007199254740994.0, 64, "9007199254740994"}, {float64(float32(0.1)), 32, "0.1"},
	}
	for _, c := range cases {
		if got := float64Text(t, c.x, c.bits); got != c.want {
			t.Errorf("Float%d %v = %s, want %s", c.bits, c.x, got, c.want)
		}
	}
	if got := string(scalarOf(t, &value.Int{V: math.MaxInt64, T: types.IntType}).raw); got != "9223372036854775807" {
		t.Errorf("Int max = %s", got)
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := floatNode(&value.Float{V: bad, T: types.FloatType}); err == nil {
			t.Errorf("%v has a wire form", bad)
		}
	}
}

func scalarOf(t *testing.T, v value.Value) *node {
	t.Helper()
	n, err := (&encoder{}).value(v, scope{})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// WIRE.md §7.3: JSON.stringify's escapes, lower-case hex, everything else raw UTF-8.
func TestStrings(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a\té/\"\x1b", `"a\té/\"\u001b"`},
		{"\b\f\n\r\\", `"\b\f\n\r\\"`},
		{"\x00\x1f\x7f", "\"\\u0000\\u001f\x7f\""},
		{"<>&\u2028\u2029 \U0001F600 ok", "\"<>&\u2028\u2029 \U0001F600 ok\""},
	}
	for _, c := range cases {
		if got := string(scalarOf(t, &value.Str{V: c.in, T: types.StringType}).raw); got != c.want {
			t.Errorf("%q = %s, want %s", c.in, got, c.want)
		}
	}
}

// WIRE.md §5.1: a Duration is an integer count of its unit.
func TestDurations(t *testing.T) {
	cases := []struct {
		ms   int64
		unit types.Unit
		want string
	}{{8000, types.UnitMs, "8000"}, {700, types.UnitMs, "700"}, {90_000, types.UnitM, ""}, {120_000, types.UnitM, "2"},
		{-3_600_000, types.UnitH, "-1"}, {172_800_000, types.UnitD, "2"}, {2000, types.UnitS, "2"}}
	for _, c := range cases {
		n, err := durNode(c.ms, c.unit)
		switch {
		case c.want == "" && err == nil:
			t.Errorf("%d ms in %s has a wire form", c.ms, c.unit)
		case c.want != "" && (err != nil || string(n.raw) != c.want):
			t.Errorf("%d ms in %s = %v, %v, want %s", c.ms, c.unit, n, err, c.want)
		}
	}
}

var jsonNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?(e[+-](0|[1-9][0-9]*))?$`)

// WIRE.md §7.2 against encoding/json's ES6 float text and a read-back of the same width.
func FuzzFloat(f *testing.F) {
	for _, x := range []float64{0, 0.1, 1e21, 1e-7, 5e-324, 1.7976931348623157e308, 123456789012345680000} {
		f.Add(math.Float64bits(x), false)
	}
	f.Add(uint64(math.Float32bits(0.1)), true)
	f.Fuzz(func(t *testing.T, raw uint64, narrow bool) {
		x, bits := math.Float64frombits(raw), float64Bits
		if narrow {
			x, bits = float64(math.Float32frombits(uint32(raw))), float32Bits
		}
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return
		}
		got := float64Text(t, x, bits)
		back, err := strconv.ParseFloat(got, bits)
		if err != nil || back != x && !(x == 0 && back == 0) {
			t.Fatalf("%s does not read back as %v (%v)", got, x, err)
		}
		if !jsonNumber.MatchString(got) {
			t.Fatalf("%s is not a canonical JSON number", got)
		}
		want := ecmascript(t, x, bits)
		if got != want {
			t.Fatalf("Float%d %v = %s, want %s", bits, x, got, want)
		}
	})
}

// ecmascript is encoding/json's float text, which is ECMAScript's; -0 is written 0 (§7.2).
func ecmascript(t *testing.T, x float64, bits int) string {
	t.Helper()
	if x == 0 {
		return "0"
	}
	var v any = x
	if bits == float32Bits {
		v = float32(x)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
