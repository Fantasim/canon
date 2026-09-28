package vm_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
)

// VIEWMODEL.md J10: integers are numbers within ±(2^53−1), decimal strings beyond.
func TestIntSafeRange(t *testing.T) {
	const safe = 1<<53 - 1
	cases := []struct {
		v    int64
		want string
	}{
		{0, `0`}, {safe, `9007199254740991`}, {-safe, `-9007199254740991`},
		{safe + 1, `"9007199254740992"`}, {-safe - 1, `"-9007199254740992"`},
		{-1 << 63, `"-9223372036854775808"`},
	}
	for _, c := range cases {
		out, err := json.Marshal(vm.Int(c.v))
		if err != nil || string(out) != c.want {
			t.Errorf("Int(%d) = %s, %v; want %s", c.v, out, err, c.want)
		}
	}
}

// VIEWMODEL.md J10: a Number reads and writes a JSON number or a decimal string, and
// refuses anything else, the absent Number included.
func TestNumber(t *testing.T) {
	for _, doc := range []string{`1`, `-0.5`, `1e21`, `"18446744073709551615"`, `"-3"`} {
		var n vm.Number
		if err := json.Unmarshal([]byte(doc), &n); err != nil {
			t.Errorf("%s: %v", doc, err)
			continue
		}
		if out, err := json.Marshal(n); err != nil || string(out) != doc {
			t.Errorf("%s re-encoded as %s, %v", doc, out, err)
		}
	}
	for _, doc := range []string{`"1.5"`, `"x"`, `true`, `null`, `{}`} {
		var n vm.Number
		if err := json.Unmarshal([]byte(doc), &n); !errors.Is(err, vm.ErrNumber) {
			t.Errorf("%s: err %v, want ErrNumber", doc, err)
		}
	}
	for _, n := range []vm.Number{{}, {Text: "01"}, {Text: "1.5", Quoted: true}} {
		if _, err := json.Marshal(n); !errors.Is(err, vm.ErrNumber) {
			t.Errorf("%+v: err %v, want ErrNumber", n, err)
		}
	}
}

// VIEWMODEL.md §12.3, §12.8: a Scalar is a string (any, "" included) or an integer.
func TestScalar(t *testing.T) {
	for _, doc := range []string{`"II_POT"`, `""`, `"12"`, `7`, `-7`} {
		var s vm.Scalar
		if err := json.Unmarshal([]byte(doc), &s); err != nil {
			t.Errorf("%s: %v", doc, err)
			continue
		}
		if out, err := json.Marshal(s); err != nil || string(out) != doc {
			t.Errorf("%s re-encoded as %s, %v", doc, out, err)
		}
	}
	for _, doc := range []string{`1.5`, `true`, `null`, `[]`} {
		var s vm.Scalar
		if err := json.Unmarshal([]byte(doc), &s); !errors.Is(err, vm.ErrScalar) {
			t.Errorf("%s: err %v, want ErrScalar", doc, err)
		}
	}
	if _, err := json.Marshal(vm.Scalar{Text: "x"}); !errors.Is(err, vm.ErrScalar) {
		t.Errorf("an unquoted non-integer encoded: %v", err)
	}
}

// VIEWMODEL.md J9: a text reference is a key string or {"text": …}, the empty neutral text
// included; anything else is refused.
func TestTextRef(t *testing.T) {
	for _, doc := range []string{`"pipeline:Potion.heal"`, `{"text":"—"}`, `{"text":""}`} {
		var r vm.TextRef
		if err := json.Unmarshal([]byte(doc), &r); err != nil {
			t.Errorf("%s: %v", doc, err)
			continue
		}
		if out, err := json.Marshal(r); err != nil || string(out) != doc {
			t.Errorf("%s re-encoded as %s, %v", doc, out, err)
		}
	}
	for _, doc := range []string{`""`, `{}`, `{"text":"a","key":"b"}`, `null`, `1`} {
		var r vm.TextRef
		if err := json.Unmarshal([]byte(doc), &r); !errors.Is(err, vm.ErrTextRef) {
			t.Errorf("%s: err %v, want ErrTextRef", doc, err)
		}
	}
	text := "t"
	for _, r := range []vm.TextRef{{}, {Key: "p:k", Text: &text}} {
		if _, err := json.Marshal(r); !errors.Is(err, vm.ErrTextRef) {
			t.Errorf("%+v: err %v, want ErrTextRef", r, err)
		}
	}
}
