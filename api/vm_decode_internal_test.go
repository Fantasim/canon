package canon

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
)

// API.md R10 (ADR-0006): Decode refuses a member named by case fold only, and a null out of a J10 value.
func TestViewModelDecodeStrict(t *testing.T) {
	const field = `{"types":{"p.T":{"kind":"record","name":"T","fields":[{"name":"f","type":{"kind":"bool"},` +
		`"default":null,"wire":{"name":"f"}}]}}}`
	cases := []struct {
		name, doc string
		want      error
		at        string
	}{
		{"exact members", `{"$schema":"canon-vm/1","package":"p","requires":[]}`, nil, ""},
		{"null J10 value", field, nil, ""},
		{"unknown member skipped (J6)", `{"package":"p","later":{"Kind":null}}`, nil, ""},
		{"wrong-case member", `{"Package":"p"}`, errDecodeCase, `"/Package"`},
		{"wrong-case nested", `{"types":{"p.T":{"Kind":"record"}}}`, errDecodeCase, `"/types/p.T/Kind"`},
		{"wrong-case text reference (J9)", `{"types":{"p/T":{"kind":"enum","help":{"Text":"x"}}}}`, errDecodeCase, `"/types/p~1T/help/Text"`},
		{"null map", `{"types":null}`, errDecodeNull, `"/types"`},
		{"null element", `{"requires":["a",null]}`, errDecodeNull, `"/requires/1"`},
		{"null string", `{"package":null}`, errDecodeNull, `"/package"`},
		{"null text reference", `{"types":{"p.T":{"kind":"enum","help":null}}}`, errDecodeNull, `"/types/p.T/help"`},
		{"null number", `{"types":{"p.T":{"kind":"record","params":[{"name":"x","type":{"kind":"int","min":null}}]}}}`, errDecodeNull, `"/types/p.T/params/0/type/min"`},
	}
	for _, c := range cases {
		m := &ViewModel{Package: "p", data: []byte(c.doc)}
		var doc vm.ViewModel
		err := m.Decode(&doc)
		switch {
		case c.want == nil && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.want != nil && (!errors.Is(err, c.want) || !strings.Contains(err.Error(), c.at)):
			t.Errorf("%s: %v, want %v at %s", c.name, err, c.want, c.at)
		}
	}
}

// embedInner and embedLang are embedded below, once tagged with a name and once with options only.
type embedInner struct {
	Schema string
}

type embedLang struct {
	Lang string `json:"language"`
}

type embedDoc struct {
	embedInner `json:"inner"`
	embedLang  `json:",omitempty"`
	Package    string `json:"package"`
}

// API.md §5.4: Decode reads embedded fields as encoding/json does (a tag name stops promotion).
func TestViewModelDecodeEmbedded(t *testing.T) {
	cases := []struct {
		doc  string
		want error
	}{
		{`{"schema":"x","inner":{"Schema":"y"}}`, nil},
		{`{"inner":{"schema":"y"}}`, errDecodeCase},
		{`{"language":"en","EmbedLang":{}}`, nil},
		{`{"Language":"en"}`, errDecodeCase},
		{`{"Package":"p"}`, errDecodeCase},
	}
	for _, c := range cases {
		var doc embedDoc
		m := &ViewModel{Package: "p", data: []byte(c.doc)}
		if err := m.Decode(&doc); !errors.Is(err, c.want) && (c.want != nil || err != nil) {
			t.Errorf("%s: %v, want %v", c.doc, err, c.want)
		}
	}
}

// Decode into an interface target keeps encoding/json's reading: no struct, no strict names.
func TestViewModelDecodeAny(t *testing.T) {
	m := &ViewModel{Package: "p", data: []byte(`{"Package":null,"types":{}}`)}
	var doc map[string]any
	if err := m.Decode(&doc); err != nil || len(doc) != len([]string{"Package", "types"}) {
		t.Errorf("Decode into a map: %v %v", doc, err)
	}
	var raw json.RawMessage
	if err := m.Decode(&raw); err != nil || string(raw) != string(m.data) {
		t.Errorf("Decode into RawMessage: %q %v", raw, err)
	}
	var notPointer vm.ViewModel
	var invalid *json.InvalidUnmarshalError
	if err := m.Decode(notPointer); !errors.As(err, &invalid) {
		t.Errorf("Decode into a value: %v, want encoding/json's refusal", err)
	}
}
