package wire_test

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// gen draws random values with the JSON encoding/json should decode them to, an oracle built
// without the encoder: numbers are compared by value, objects as Go maps.
type gen struct {
	r    *rand.Rand
	enum *types.EnumType
}

type drawn struct {
	v    value.Value
	want any
}

func (g *gen) draw(depth int) drawn {
	kinds := 8
	if depth > 0 {
		kinds = 11
	}
	switch g.r.IntN(kinds) {
	case 0:
		b := g.r.IntN(2) == 0
		return drawn{boolean(b), b}
	case 1:
		i := int64(g.r.Uint64())
		return drawn{num(i), json.Number(strconv.FormatInt(i, 10))}
	case 2:
		return g.float()
	case 3, 4:
		s := g.string()
		return drawn{str(s), s}
	case 5:
		ms := g.r.Int64N(2*types.DurationLimit) - types.DurationLimit
		return drawn{dur(ms), json.Number(strconv.FormatInt(ms, 10))}
	case 6:
		i := g.r.IntN(len(g.enum.Members))
		return drawn{member(g.enum, i), g.enum.Members[i].Wire}
	case 7:
		return drawn{none(nil), nil}
	case 8:
		return g.list(depth)
	case 9:
		return g.mapOf(depth)
	}
	return g.record(depth)
}

func (g *gen) float() drawn {
	x := math.Float64frombits(g.r.Uint64())
	if math.IsNaN(x) || math.IsInf(x, 0) {
		x = g.r.NormFloat64() * math.Pow(10, float64(g.r.IntN(40)-20))
	}
	return drawn{flt(x), x}
}

// string mixes ASCII, the characters §7.3 escapes and non-ASCII, U+2028 included.
func (g *gen) string() string {
	alphabet := []rune("ab\"\\/\b\f\n\r\t\x00\x01\x1f\x7f<>&é\u2028\u2029\U0001F600")
	var b strings.Builder
	for range g.r.IntN(12) {
		b.WriteRune(alphabet[g.r.IntN(len(alphabet))])
	}
	return b.String()
}

func (g *gen) list(depth int) drawn {
	l := &value.List{T: &types.ListType{Elem: types.AnyType}}
	want := []any{}
	for range g.r.IntN(4) {
		d := g.draw(depth - 1)
		l.Elems, want = append(l.Elems, d.v), append(want, d.want)
	}
	return drawn{l, want}
}

func (g *gen) mapOf(depth int) drawn {
	m := &value.Map{T: &types.MapType{Key: types.StringType, Value: types.AnyType}}
	want := map[string]any{}
	for range g.r.IntN(4) {
		k := g.string()
		if _, dup := want[k]; dup {
			continue
		}
		d := g.draw(depth - 1)
		m.Keys, m.Vals, want[k] = append(m.Keys, str(k)), append(m.Vals, d.v), d.want
	}
	return drawn{m, want}
}

func (g *gen) record(depth int) drawn {
	t := &types.RecordType{Pkg: "p", Name: "R"}
	r := rec(t)
	want := map[string]any{}
	for i := range g.r.IntN(5) {
		d := g.draw(depth - 1)
		name := "f" + strconv.Itoa(i)
		t.Fields = append(t.Fields, field(name, types.AnyType))
		r.Fields, want[name] = append(r.Fields, d.v), d.want
	}
	return drawn{r, want}
}

// WIRE.md §7: valid, deterministic JSON that decodes to the values, compact or pretty.
func TestEncodeProperties(t *testing.T) {
	g := &gen{r: rand.New(rand.NewPCG(1, 2)), enum: enum("p", "E", "a", "series-1", "é")}
	for i := range 2000 {
		d := g.draw(3)
		one, err := (&wire.Document{Schema: "p.v@00000000", Kind: types.Record, V: d.v}).Encode()
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		two, _ := (&wire.Document{Schema: "p.v@00000000", Kind: types.Record, V: d.v}).Encode()
		rows, err := (&wire.Document{Schema: "p.v@00000000", Kind: types.List, V: list(types.AnyType, d.v)}).Encode()
		if err != nil || !bytes.Equal(one, two) {
			t.Fatalf("case %d: not deterministic (%v)", i, err)
		}
		checkLayout(t, one)
		checkLayout(t, rows)
		var doc struct{ Value any }
		var tab struct{ Rows []any }
		if decode(one, &doc) != nil || decode(rows, &tab) != nil || len(tab.Rows) != 1 {
			t.Fatalf("case %d: invalid JSON:\n%s\n%s", i, one, rows)
		}
		if !same(doc.Value, d.want) || !same(tab.Rows[0], d.want) {
			t.Fatalf("case %d: decodes to\n%#v\n%#v\nwant\n%#v", i, doc.Value, tab.Rows[0], d.want)
		}
	}
}

func decode(b []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return dec.Decode(into)
}

// checkLayout holds WIRE.md §7.1: UTF-8, LF only, one final LF, space indentation.
func checkLayout(t *testing.T, b []byte) {
	t.Helper()
	s := string(b)
	if !utf8.ValidString(s) || strings.Contains(s, "\r") || !strings.HasSuffix(s, "}\n") || strings.HasSuffix(s, "\n\n") {
		t.Fatalf("bad file layout:\n%q", s)
	}
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " "), "\t") || strings.HasSuffix(line, " ") {
			t.Fatalf("bad line %q", line)
		}
	}
}

// same compares a decoded document with the oracle; numbers by value, floats read as float64.
func same(got, want any) bool {
	switch w := want.(type) {
	case float64:
		n, ok := got.(json.Number)
		f, err := strconv.ParseFloat(string(n), 64)
		return ok && err == nil && (f == w || f == 0 && w == 0)
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !same(g[i], w[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for k, v := range w { //canon:unordered a comparison, each key checked alone
			if !same(g[k], v) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(got, want)
}
