package viewgen

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
)

// TestSentinels is go.md 3: every error Write can return wraps a named sentinel, never bare.
func TestSentinels(t *testing.T) {
	s := "s"
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"nil model", writeErr(nil), errNil},
		{"TextRef neither", textRefErr(vm.TextRef{}), errTextRef},
		{"TextRef both", textRefErr(vm.TextRef{Key: "k", Text: &s}), errTextRef},
		{"raw malformed", rawErr(`not json`), errRaw},
		{"raw trailing", rawErr(`1 2`), errRaw},
		{"raw empty", rawErr(``), errRaw},
		{"unsupported Go type", valueErr(complex(1, 2)), errType},
		{"non-string map key", mapErr(map[int]string{1: "a"}), errMapKey},
		{"Number not a number", numberErr(vm.Number{Text: "abc"}), vm.ErrNumber},
		{"Number zero", numberErr(vm.Number{}), vm.ErrNumber},
		{"Number -0", numberErr(vm.Number{Text: "-0"}), errNumberText},
		{"Number 1.50", numberErr(vm.Number{Text: "1.50"}), errNumberText},
		{"Number unquoted beyond safe range", numberErr(vm.Number{Text: maxSafeIntText + "0"}), errNumberText},
		{"Number 2^53+1 refused, not exactly representable", numberErr(vm.Number{Text: "9007199254740993"}), errNumberText},
		{"Number 2^53+2 accepted, WIRE 7.2 step 4 bare digits", numberErr(vm.Number{Text: "9007199254740994"}), nil},
		{"Number 1e20 accepted, WIRE 7.2 step 4 bare digits", numberErr(vm.Number{Text: "100000000000000000000"}), nil},
		{"Number quoted in range refused", numberErr(vm.Number{Text: "5", Quoted: true}), errNumberText},
		{"Number quoted float refused", numberErr(vm.Number{Text: "1.5", Quoted: true}), vm.ErrNumber},
		{"Number quoted beyond range accepted", numberErr(vm.Number{Text: maxSafeIntText + "0", Quoted: true}), nil},
		{"Scalar not a scalar", scalarErr(vm.Scalar{Text: "a b"}), vm.ErrScalar},
		{"Scalar zero (Member.Wire empty)", scalarErr(vm.Scalar{}), vm.ErrScalar},
		{"Scalar unquoted beyond safe range refused", scalarErr(vm.Scalar{Text: "9007199254740993"}), errNumberText},
		{"Scalar quoted float text accepted (any text)", scalarErr(vm.Scalar{Text: "1.5", Quoted: true}), nil},
		{"Unit{} (a zero Number, required)", unitErr(vm.Unit{}), vm.ErrNumber},
		{"raw non-canonical float", rawErr(`-0.0`), errNumberText},
		{"raw bare integer beyond safe range", rawErr(maxSafeIntText + "0"), errNumberText},
		{"raw big float as bare digits accepted", rawErr(`9007199254740994`), nil},
		{"raw duplicate key", rawErr(`{"a":1,"a":2}`), errRaw},
		{"raw invalid UTF-8", rawErr("\xff"), errRaw},
		{"raw leading UTF-8 BOM", rawErr("\xef\xbb\xbf1"), errRaw},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !errors.Is(c.err, c.want) {
				t.Errorf("got %v, want it to wrap %v", c.err, c.want)
			}
		})
	}
}

func writeErr(m *vm.ViewModel) error { _, err := Write(m); return err }
func textRefErr(r vm.TextRef) error  { _, err := buildTextRef(r); return err }
func rawErr(text string) error       { _, err := buildRaw(json.RawMessage(text)); return err }
func valueErr(v any) error           { _, err := buildValue(reflect.ValueOf(v)); return err }
func mapErr(v any) error             { _, err := buildMap(reflect.ValueOf(v)); return err }
func numberErr(n vm.Number) error    { _, err := buildNumber(n); return err }
func scalarErr(s vm.Scalar) error    { _, err := buildScalarValue(s); return err }
func unitErr(u vm.Unit) error        { _, err := buildValue(reflect.ValueOf(u)); return err }
