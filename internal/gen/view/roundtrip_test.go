package viewgen_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
)

// potionGolden is the pre-drafted golden of VIEWMODEL.md 15.1, reproduced byte for byte since
// api.ViewModel().JSON() and `emit view` both call Write; a mismatch is reported, never fixed
// by editing the golden.
const potionGolden = "../../../examples/pipeline/expected/potion.view.json"

// TestPotionRoundTrip is VIEWMODEL.md J1-J3, J10: decoding the golden into vm.ViewModel and
// writing it back reproduces the same bytes.
func TestPotionRoundTrip(t *testing.T) {
	want, err := os.ReadFile(potionGolden)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(want))
	dec.DisallowUnknownFields()
	var m vm.ViewModel
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	got, err := viewgen.Write(&m)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Write of the decoded golden differs:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}
