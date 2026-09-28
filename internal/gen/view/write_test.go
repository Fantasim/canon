package viewgen_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// decode reads the case's input.json into a vm.ViewModel, strict about unknown members like
// api/vm's own tests (VIEWMODEL.md V2).
func decode(t *testing.T, c golden.Case) *vm.ViewModel {
	t.Helper()
	var input []byte
	for _, f := range c.Archive.Files {
		if f.Name == "input.json" {
			input = f.Data
		}
	}
	if input == nil {
		t.Fatalf("%s: no input.json", c.Path)
	}
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.DisallowUnknownFields()
	var m vm.ViewModel
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("%s: decode input.json: %v", c.Path, err)
	}
	return &m
}

// write is the golden.Run callback every case below shares: decode, then Write.
func write(t *testing.T, c golden.Case) []byte {
	t.Helper()
	m := decode(t, c)
	out, err := viewgen.Write(m)
	if err != nil {
		t.Fatalf("%s: Write: %v", c.Path, err)
	}
	return out
}

// TestEscaping is WIRE.md 7.3: control escapes, and `<`, `>`, `&`, U+2028, U+2029 raw.
func TestEscaping(t *testing.T) {
	golden.Run(t, "testdata/escaping.txtar", write)
}

// TestNumbers is WIRE.md 7.2 (the float table, `-0`) through a Field.Default (VIEWMODEL.md
// J10), re-emitted from json.RawMessage rather than passed through raw.
func TestNumbers(t *testing.T) {
	golden.Run(t, "testdata/numbers.txtar", write)
}

// TestBigNumbers is VIEWMODEL.md J10: an integer beyond ±(2^53-1) as a decimal string, in a
// vm.Number (Control.min/max) and a vm.Scalar (an enum member's wire code).
func TestBigNumbers(t *testing.T) {
	golden.Run(t, "testdata/bignum.txtar", write)
}

// TestPresence is VIEWMODEL.md J3: an absent optional member vs. an "always present" one kept
// even empty, and an empty non-nil optional slice staying present (api/vm/doc.go).
func TestPresence(t *testing.T) {
	golden.Run(t, "testdata/presence.txtar", write)
}

// TestMapKeyOrder is VIEWMODEL.md J2: a program-named object's members sorted by byte order,
// including non-ASCII names.
func TestMapKeyOrder(t *testing.T) {
	golden.Run(t, "testdata/mapkeys.txtar", write)
}

// TestNestedUnion is VIEWMODEL.md J10 (a variant value's `$case` first, a nested map in map
// order) and 12.3 (a `union` type expression), through a Field.Default and a Field.Type.
func TestNestedUnion(t *testing.T) {
	golden.Run(t, "testdata/nestedunion.txtar", write)
}
