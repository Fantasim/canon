package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md E24, API.md E25, API.md E26: an Edit reads and writes the JSON form of API.md 8.8: a
// wire value as `value`, `null` as None written back as `source`, a Canon literal as `source`,
// a key as a string or an integer; each round trip is exact.
func TestOpJSON(t *testing.T) {
	cases := []struct{ in, out string }{
		{`{"op":"set","path":"a:config.port","value":9000}`, `{"op":"set","path":"a:config.port","value":9000}`},
		{`{"op":"set","path":"a:config.name","value":null}`, `{"op":"set","path":"a:config.name","source":"none"}`},
		{`{"op":"addEntry","path":"a:statuses","key":"blocked","source":"{ label: \"Blocked\" }"}`, ""},
		{`{"op":"move","path":"a:picked[1]","index":0}`, ""},
		{`{"op":"rename","path":"a:statuses.open","key":"opened"}`, ""},
		{`{"op":"insert","path":"a:picked","index":1,"source":"statuses.open"}`, ""},
		{`{"op":"setCase","path":"a:shape","case":"circle"}`, ""},
		{`{"op":"reset","path":"a:config.port"}`, ""},
		{`{"op":"retire","path":"a:codes.first"}`, ""},
		{`{"op":"addEntry","path":"a:byCode","key":3,"value":"x"}`, ""},
	}
	for _, c := range cases {
		var op canon.Op
		if err := json.Unmarshal([]byte(c.in), &op); err != nil {
			t.Errorf("%s: %v", c.in, err)
			continue
		}
		out, err := json.Marshal(op)
		want := c.out
		if want == "" {
			want = c.in
		}
		if err != nil || string(out) != want {
			t.Errorf("%s: %s, %v, want %s", c.in, out, err, want)
		}
	}
	e := `{"base":"r1:5f0c","ops":[{"op":"set","path":"a:config.port","value":10}],"dryRun":true}`
	var edit canon.Edit
	if err := json.Unmarshal([]byte(e), &edit); err != nil || !edit.DryRun || edit.Base != "r1:5f0c" {
		t.Fatalf("Edit: %+v, %v", edit, err)
	}
	if out, err := json.Marshal(edit); err != nil || string(out) != e {
		t.Errorf("Edit round trip: %s, %v", out, err)
	}
}

// API.md E24, API.md E26: an unknown operation or member, a value given twice, a missing one, an
// Op of no kind, and a value with no Canon literal are refused.
func TestOpJSONRefused(t *testing.T) {
	for _, in := range []string{
		`{"op":"drop","path":"a:x"}`,
		`{"op":"set","path":"a:x","value":1,"source":"1"}`,
		`{"op":"set","path":"a:x"}`,
		`{"op":"set","path":"a:x","value":1,"extra":true}`,
		`{"op":"reset","path":"a:x","index":2}`,
		`{"op":"set","path":"a:x","value":1,"value":2}`,
		`[]`,
	} {
		var op canon.Op
		if err := json.Unmarshal([]byte(in), &op); err == nil {
			t.Errorf("%s: accepted as %+v", in, op)
		}
	}
	for _, op := range []canon.Op{{Kind: "drop", Path: "a:x"}, canon.Set("a:x", canon.Float(math.NaN()))} {
		if out, err := json.Marshal(op); err == nil {
			t.Errorf("%+v: written as %s", op, out)
		}
	}
}

// API.md E25: a key read from JSON is a path key, so an edit sent as JSON adds and renames
// entries as one built with Key does.
func TestOpJSONEdit(t *testing.T) {
	p, _ := openEdit(t, nil)
	var e canon.Edit
	in := `{"base":"","ops":[` +
		`{"op":"addEntry","path":"a:statuses","key":"blocked","source":"{ label: \"Blocked\" }"},` +
		`{"op":"rename","path":"a:statuses.blocked","key":"held"}],"dryRun":true}`
	if err := json.Unmarshal([]byte(in), &e); err != nil {
		t.Fatal(err)
	}
	res, err := p.Edit(context.Background(), e)
	if err != nil || len(res.Changes) != 1 || len(lines(string(res.Changes[0].After), "held {")) != 1 {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
}

// API.md E24, API.md X1 (log-2026-09-29 M4 U5b-r): an Op or Edit JSON that does not decode is a
// *PathError wrapping ErrBadOp, its Detail the codec's reason, its Op the index of the op at fault.
func TestOpJSONBadOp(t *testing.T) {
	cases := []struct {
		in string
		op int
		e  bool
	}{
		{`{"op":"drop","path":"a:x"}`, -1, false},
		{`{"base":"","ops":[{"op":"reset","path":"a:x"},{"op":"set","path":"a:x"}]}`, 1, true},
		{`{"base":3,"ops":[]}`, -1, true},
	}
	for _, c := range cases {
		var err error
		if c.e {
			var e canon.Edit
			err = json.Unmarshal([]byte(c.in), &e)
		} else {
			var op canon.Op
			err = json.Unmarshal([]byte(c.in), &op)
		}
		var pe *canon.PathError
		if !errors.As(err, &pe) || !errors.Is(err, canon.ErrBadOp) || pe.Op != c.op || pe.Detail == "" {
			t.Errorf("%s: %v", c.in, err)
		}
	}
}
