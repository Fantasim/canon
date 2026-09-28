package vm_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
)

const potionGolden = "../../examples/pipeline/expected/potion.view.json"

// decodeStrict decodes data into v, refusing a member no field of v names (VIEWMODEL.md V2).
func decodeStrict(t *testing.T, data []byte, v any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatal(err)
	}
}

// tree is data as a generic JSON value, numbers kept as written.
func tree(t *testing.T, data []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// API.md R10, VIEWMODEL.md J1-J3: the pipeline view model decodes into the structs, strictly,
// and encodes back to the same document (every member kept, none added).
func TestPotionRoundTrip(t *testing.T) {
	data, err := os.ReadFile(potionGolden)
	if err != nil {
		t.Fatal(err)
	}
	var model vm.ViewModel
	decodeStrict(t, data, &model)
	if got := model.Types["pipeline.Potion"].Fields[3].Type.Min; got != vm.Int(0) {
		t.Errorf("cooldown min = %+v, want the present 0", got)
	}
	if model.I18N.Languages["fr"].Files == nil || *model.I18N.Languages["fr"].Missing != 14 {
		t.Errorf("fr = %+v, want files [] and missing 14 present", model.I18N.Languages["fr"])
	}
	out, err := json.Marshal(&model)
	if err != nil {
		t.Fatal(err)
	}
	if want, got := tree(t, data), tree(t, out); !reflect.DeepEqual(want, got) {
		t.Errorf("re-encoded model differs:\n%s", out)
	}
}

// VIEWMODEL.md J3, J10, §12.3, §12.10: a member present with its zero value stays present.
func TestZeroValuesStayPresent(t *testing.T) {
	cases := []struct {
		doc string
		v   any
	}{
		{`{"kind":"typeFunction","name":"F","params":[],"select":"","branches":[],"drivers":{}}`, &vm.TypeDef{}},
		{`{"kind":"int","bits":8,"signed":false,"min":0}`, &vm.TypeExpr{}},
		{`{"kind":"ref","element":"$define","keyType":"int","count":0,"active":0,"sibling":{"up":0,"field":"f"}}`, &vm.TypeExpr{}},
		{`{"files":[],"missing":0,"texts":{}}`, &vm.Language{}},
		{`{"texts":{}}`, &vm.Language{}},
		{`{"name":"n","type":{"kind":"optional","of":{"kind":"bool"}},"default":null,"wire":{"name":"n","none":null}}`, &vm.Field{}},
		{`{"severity":"error","code":"X","pointer":"","package":"p","message":"m"}`, &vm.Finding{}},
	}
	for _, c := range cases {
		decodeStrict(t, []byte(c.doc), c.v)
		out, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tree(t, []byte(c.doc)), tree(t, out)) {
			t.Errorf("%s\nre-encoded as\n%s", c.doc, out)
		}
	}
}

// API.md F5, VIEWMODEL.md J10, J15, §12.5: moreFrames and decimal-string bounds round-trip.
func TestMoreFramesAndDecimalBounds(t *testing.T) {
	cases := []struct {
		doc string
		v   any
	}{
		{`{"severity":"error","code":"X","package":"p","message":"m","stack":[{"fn":"f","file":"a.canon","line":1,"col":1,"endLine":1,"endCol":2}],"moreFrames":3}`, &vm.Finding{}},
		{`{"kind":"number","min":"-9223372036854775808","max":"18446744073709551615"}`, &vm.Control{}},
		{`{"kind":"slider","min":0,"max":0.5}`, &vm.Control{}},
	}
	for _, c := range cases {
		decodeStrict(t, []byte(c.doc), c.v)
		out, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tree(t, []byte(c.doc)), tree(t, out)) {
			t.Errorf("%s\nre-encoded as\n%s", c.doc, out)
		}
	}
	var f vm.Finding
	decodeStrict(t, []byte(cases[0].doc), &f)
	if f.MoreFrames != 3 {
		t.Errorf("moreFrames = %d, want 3", f.MoreFrames)
	}
}

// API.md R10: the documented limits of decoding with encoding/json, which ViewModel.Decode's
// strict check (a later unit) closes: names match case-insensitively, and null is taken for a
// pointer, slice or map member, a required one included.
func TestEncodingJSONLimits(t *testing.T) {
	var te vm.TypeExpr
	decodeStrict(t, []byte(`{"KIND":"int","Bits":8,"signed":null}`), &te)
	if te.Kind != "int" || te.Bits != 8 || te.Signed != nil {
		t.Errorf("got %+v, want kind int, bits 8, signed absent", te)
	}
	var c vm.Case
	decodeStrict(t, []byte(`{"name":"C","wire":"W","label":"p:k","fields":null}`), &c)
	if c.Fields != nil {
		t.Errorf("fields = %v, want nil from null", c.Fields)
	}
}

// VIEWMODEL.md V2: the structs are as strict as the schema about member names.
func TestUnknownMemberRefused(t *testing.T) {
	dec := json.NewDecoder(bytes.NewReader([]byte(`{"kind":"bool","nope":1}`)))
	dec.DisallowUnknownFields()
	var te vm.TypeExpr
	if err := dec.Decode(&te); err == nil {
		t.Error("an unknown member decoded")
	}
}
