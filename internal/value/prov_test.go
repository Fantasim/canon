package value_test

import (
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// provBytes is the size of a Prov on a 64-bit target: Kind, the position mark and padding, a
// 12-byte span, Via and Ext (DECISIONS 373).
const provBytes = 32

// DECISIONS 373, 379: the position mark sits in Prov's padding, so a Prov stays 32 bytes.
func TestProvSizeUnchanged(t *testing.T) {
	if got := reflect.TypeFor[value.Prov]().Size(); got != provBytes {
		t.Fatalf("value.Prov is %d bytes, want %d", got, provBytes)
	}
}

// EVALUATION.md §13.1: IsPosition is a marked scalar of four kinds; IsEntry an entry or a ref.
func TestPositionPredicates(t *testing.T) {
	marked := &value.Prov{Kind: value.ProvComputed, Position: true}
	positions := []struct {
		name string
		v    value.Value
		want bool
	}{
		{"marked Int", &value.Int{V: 1, T: types.IntType, P: marked}, true},
		{"marked Float", &value.Float{V: 1, T: types.FloatType, P: marked}, true},
		{"marked Duration", &value.Dur{Ms: 1, P: marked}, true},
		{"marked String", &value.Str{V: "1", T: types.StringType, P: marked}, true},
		{"unmarked Int", &value.Int{V: 1, T: types.IntType, P: lit}, false},
		{"Int without provenance", &value.Int{V: 1, T: types.IntType}, false},
		{"marked Bool", &value.Bool{V: true, P: marked}, false},
		{"nil", nil, false},
	}
	for _, c := range positions {
		if got := value.IsPosition(c.v); got != c.want {
			t.Errorf("IsPosition(%s) = %v, want %v", c.name, got, c.want)
		}
	}
	entries := []struct {
		name string
		v    value.Value
		want bool
	}{
		{"entry", &value.Record{T: status, Ident: &value.Identity{Coll: statuses, Key: value.Key{S: "open"}}}, true},
		{"ref", &value.Ref{T: refStatus, Key: value.Key{S: "open"}}, true},
		{"plain record", &value.Record{T: status}, false},
		{"Int", &value.Int{V: 1, T: types.IntType}, false},
		{"nil", nil, false},
	}
	for _, c := range entries {
		if got := value.IsEntry(c.v); got != c.want {
			t.Errorf("IsEntry(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
