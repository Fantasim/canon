package gogen_test

import (
	"strings"
	"testing"
)

// The edge enums of TestEnumEdgeCases: equal wires, a wire per name, no member at all.
const (
	edgeModule  = "example.com/edge"
	edgeClosed  = "closed/closed.gen.go"
	edgeOpen    = "open/open.gen.go"
	edgeWiresOf = "enumWires_"
)

// edgeFixture has a baked package (T with equal wires, E with every wire its name, Z with no member) and a types package opening the same three, T and Z with codes.
func edgeFixture() *fixture {
	both := func(name string, members []string, codes string, codeValues []int64) enumSpec {
		return enumSpec{Name: name, Members: members, Codes: codes, CodeValues: codeValues}
	}
	return &fixture{Module: edgeModule, Packages: []*pkgSpec{
		{
			Name: "edge.closed", Dir: "edge/closed",
			Enums: []enumSpec{both("T", []string{"a=x", "b=x"}, "", nil), both("E", []string{"p", "q"}, "UInt8", []int64{9, 4}), both("Z", nil, "", nil), both("ZC", nil, "UInt8", nil)},
			Emit:  emitSpec{Out: "out/", Dir: "edge/closed", Import: edgeModule + "/closed", Package: "closed", Mode: "baked"},
		},
		{
			Name: "edge.open", Dir: "edge/open",
			Enums: []enumSpec{both("T", []string{"a=x", "b=x"}, "UInt8", []int64{1, 2}), both("Z", nil, "", nil), both("ZC", nil, "UInt8", nil)},
			Emit:  emitSpec{Out: "out/", Dir: "edge/open", Import: edgeModule + "/open", Package: "open", Mode: "types", Open: []string{"T", "Z", "ZC"}},
		},
	}}
}

const edgeClosedSmoke = `package closed_test

import (
	"testing"

	closed "example.com/edge/closed"
)

func TestEdge(t *testing.T) {
	if got, ok := closed.ParseT("x"); !ok || got != closed.TA {
		t.Errorf("ParseT(x) = %v, %v, want TA", got, ok)
	}
	if closed.TA.String() != "a" || closed.TB.String() != "b" || closed.TA.Wire() != "x" || closed.TB.Wire() != "x" {
		t.Errorf("T: %q %q %q %q", closed.TA.String(), closed.TB.String(), closed.TA.Wire(), closed.TB.Wire())
	}
	if closed.EP.Wire() != "p" || closed.EQ.Wire() != "q" || closed.EQ.Code() != 4 {
		t.Errorf("E: %q %q %d", closed.EP.Wire(), closed.EQ.Wire(), closed.EQ.Code())
	}
	if got, ok := closed.EFromCode(4); !ok || got != closed.EQ {
		t.Errorf("EFromCode(4) = %v, %v", got, ok)
	}
	if got, ok := closed.ParseE("q"); !ok || got != closed.EQ {
		t.Errorf("ParseE(q) = %v, %v", got, ok)
	}
	if got := closed.E(7).Wire(); got != "E(7)" {
		t.Errorf("E(7).Wire() = %q", got)
	}
	if closed.Z(0).String() != "Z(0)" || closed.Z(1).Wire() != "Z(1)" {
		t.Errorf("Z: %q %q", closed.Z(0).String(), closed.Z(1).Wire())
	}
	if got, ok := closed.ParseZ("x"); ok || got != 0 {
		t.Errorf("ParseZ(x) = %v, %v", got, ok)
	}
	if got, ok := closed.ZCFromCode(0); ok || got != 0 {
		t.Errorf("ZCFromCode(0) = %v, %v", got, ok)
	}
	for range closed.ZMembers() {
		t.Error("Z has a member")
	}
	if _, err := closed.Z(0).MarshalText(); err == nil {
		t.Error("Z(0).MarshalText succeeded")
	}
}
`

const edgeOpenSmoke = `package open_test

import (
	"testing"

	open "example.com/edge/open"
)

func TestEdge(t *testing.T) {
	if open.TA != open.TB {
		t.Fatalf("TA %q and TB %q differ", open.TA, open.TB)
	}
	if got, ok := open.ParseT("x"); !ok || got != open.TA {
		t.Errorf("ParseT(x) = %v, %v", got, ok)
	}
	if open.TB.String() != "a" || open.TB.Wire() != "x" || !open.TB.Known() {
		t.Errorf("TB: %q %q %v", open.TB.String(), open.TB.Wire(), open.TB.Known())
	}
	if code, ok := open.TB.Code(); !ok || code != 1 {
		t.Errorf("TB.Code() = %d, %v", code, ok)
	}
	if got, ok := open.TFromCode(2); !ok || got != open.TB {
		t.Errorf("TFromCode(2) = %v, %v", got, ok)
	}
	z := open.Z("w")
	if z.Known() || z.String() != "w" || z.Wire() != "w" {
		t.Errorf("Z: %v %q %q", z.Known(), z.String(), z.Wire())
	}
	if got, ok := open.ParseZ("w"); ok || got != z {
		t.Errorf("ParseZ(w) = %v, %v", got, ok)
	}
	if code, ok := open.ZC("w").Code(); ok || code != 0 {
		t.Errorf("ZC.Code() = %d, %v", code, ok)
	}
	if got, ok := open.ZCFromCode(0); ok || got != "" {
		t.Errorf("ZCFromCode(0) = %q, %v", got, ok)
	}
	for range open.ZMembers() {
		t.Error("Z has a member")
	}
}
`

// CODEGEN.md §5.2, DECISIONS 383, 393: members with equal wires compile and vet in both forms, the first declared wins Parse<E> and Known and keeps its Canon name, an enum whose wires are its names has no wires table, and an enum without members compiles, vets and answers as an enum with every value outside it.
func TestEnumEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	_, files := buildWorld(t, edgeFixture())
	closed := string(files[edgeClosed])
	for _, want := range []string{edgeWiresOf + "T =", "func ZMembers("} {
		if !strings.Contains(closed, want) {
			t.Errorf("the closed file has no %q", want)
		}
	}
	for _, unwanted := range []string{edgeWiresOf + "E ", edgeWiresOf + "Z "} {
		if strings.Contains(closed, unwanted) {
			t.Errorf("the closed file holds %q, but the wires are the names", unwanted)
		}
	}
	mod := t.TempDir()
	writeNoncedModule(t, mod, edgeModule, files, map[string][]byte{"closed/smoke_test.go": []byte(edgeClosedSmoke), "open/smoke_test.go": []byte(edgeOpenSmoke)})
	goCmd(t, mod, "vet", "./...")
	goCmd(t, mod, "test", "-count=1", "./...")
}
