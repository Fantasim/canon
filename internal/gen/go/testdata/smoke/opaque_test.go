package opaque_test

import (
	"bytes"
	"database/sql/driver"
	"encoding"
	"encoding/gob"
	"encoding/json"
	"testing"

	opaque "example.com/data/demo/opaque/out/go"
)

type holder struct {
	S opaque.StatusID
	L []opaque.StatusID
	T opaque.Tone
	M map[opaque.Tone]int
	C opaque.Coded
}

// Smoke test of an opaque id and of the text encoders of an enum (CODEGEN.md §5.2, §5.3; DECISIONS 379).
func TestIDTypeOpaque(t *testing.T) {
	h := holder{S: opaque.StatusIDTaken, L: []opaque.StatusID{opaque.StatusIDOpen}, T: opaque.ToneSeries1, M: map[opaque.Tone]int{opaque.ToneSeries1: 1}, C: opaque.CodedHigh}
	got, err := json.Marshal(h)
	const want = `{"S":"taken","L":["open"],"T":"series-1","M":{"series-1":1},"C":200}`
	if err != nil || string(got) != want {
		t.Fatalf("json.Marshal = %s, %v; want %s", got, err, want)
	}
	var back holder
	if err := json.Unmarshal(got, &back); err != nil || back.S != h.S || back.L[0] != h.L[0] || back.T != h.T || back.M[opaque.ToneSeries1] != 1 || back.C != h.C {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	if opaque.StatusIDTaken.Position() != 1 || opaque.StatusIDTaken.String() != "taken" {
		t.Error("Position or String")
	}
	var id opaque.StatusID
	if err := id.UnmarshalText([]byte("nope")); err == nil || err.Error() != `StatusID: no entry "nope"` {
		t.Errorf("unknown key: %v", err)
	}
	if _, err := driver.DefaultParameterConverter.ConvertValue(opaque.StatusIDTaken); err == nil {
		t.Error("database/sql stored an id")
	}
	if err := gob.NewEncoder(&bytes.Buffer{}).Encode(opaque.StatusIDTaken); err == nil {
		t.Error("gob wrote an id")
	}
}

// CODEGEN.md §5.2, §5.5 (DECISIONS 379): an enum without @codes writes and reads its wire value; one with @codes has no text methods.
func TestEnumMarshalText(t *testing.T) {
	var tone opaque.Tone
	if err := tone.UnmarshalText([]byte("nope")); err == nil || err.Error() != `Tone: no member "nope"` {
		t.Errorf("unknown wire value: %v", err)
	}
	if _, err := opaque.Tone(9).MarshalText(); err == nil || err.Error() != "Tone: no member 9" {
		t.Errorf("non-member value: %v", err)
	}
	if _, ok := any(opaque.CodedHigh).(encoding.TextMarshaler); ok {
		t.Error("an @codes enum has MarshalText")
	}
	if _, ok := any(new(opaque.Coded)).(encoding.TextUnmarshaler); ok {
		t.Error("an @codes enum has UnmarshalText")
	}
}
