package gogen_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// The retired fixture: a closed enum and a @codes enum with retired members, an open enum with one, a baked table with a retired entry.
const (
	retiredModule = "example.com/retired"
	retiredClosed = "closed/closed.gen.go"
	retiredOpen   = "open/open.gen.go"
)

func retiredFixture() *fixture {
	items := valueSpec{Name: "items", Type: "table Item", Entries: []entrySpec{
		{ID: "live", Fields: map[string]json.RawMessage{"label": json.RawMessage(`"x"`)}},
		{ID: "gone", Retired: true, Fields: map[string]json.RawMessage{"label": json.RawMessage(`"y"`)}},
	}}
	return &fixture{Module: retiredModule, Packages: []*pkgSpec{
		{
			Name: "retired.closed", Dir: "retired/closed",
			Enums: []enumSpec{
				{Name: "Plain", Members: []string{"a", "b", "c"}, Retired: []string{"b"}},
				{Name: "Dup", Members: []string{"a=x", "b=x", "c"}, Retired: []string{"b"}},
				{Name: "Coded", Members: []string{"p", "q", "r"}, Codes: "UInt8", CodeValues: []int64{9, 4, 6}, Retired: []string{"p"}},
			},
			Emit:    emitSpec{Out: "out/", Dir: "retired/closed", Import: retiredModule + "/closed", Package: "closed", Mode: "baked", Values: []string{"items"}},
			Records: []recordSpec{{Name: "Item", Fields: []fieldSpec{{Name: "label", Type: "String"}}}}, Values: []valueSpec{items},
		},
		{
			Name: "retired.open", Dir: "retired/open",
			Enums: []enumSpec{
				{Name: "Opened", Members: []string{"x", "y=why"}, Codes: "UInt8", CodeValues: []int64{5, 3}, Retired: []string{"y"}},
				{Name: "OpenDup", Members: []string{"first=x", "second=x", "third", "fourth=y", "fifth=y"}, Retired: []string{"second", "fourth"}},
			},
			Emit: emitSpec{Out: "out/", Dir: "retired/open", Import: retiredModule + "/open", Package: "open", Mode: "types", Open: []string{"Opened", "OpenDup"}},
		},
	}}
}

const retiredClosedSmoke = `package closed_test

import (
	"slices"
	"testing"

	closed "example.com/retired/closed"
)

func TestRetired(t *testing.T) {
	for _, c := range []struct {
		name string
		got  bool
		want bool
	}{
		{"Plain a", closed.PlainA.Retired(), false},
		{"Plain b", closed.PlainB.Retired(), true},
		{"Plain c", closed.PlainC.Retired(), false},
		{"Plain outside", closed.Plain(200).Retired(), false},
		{"Coded p", closed.CodedP.Retired(), true},
		{"Coded q", closed.CodedQ.Retired(), false},
		{"Coded r", closed.CodedR.Retired(), false},
		{"Coded outside", closed.Coded(200).Retired(), false},
		{"Coded unused code", closed.Coded(5).Retired(), false},
		{"id live", closed.ItemIDLive.Retired(), false},
		{"id gone", closed.ItemIDGone.Retired(), true},
		{"id zero", closed.ItemID{}.Retired(), false},
	} {
		if c.got != c.want {
			t.Errorf("%s: Retired() = %v, want %v", c.name, c.got, c.want)
		}
	}
	if got := slices.Collect(closed.PlainLiveMembers()); !slices.Equal(got, []closed.Plain{closed.PlainA, closed.PlainC}) {
		t.Errorf("PlainLiveMembers = %v", got)
	}
	if got := slices.Collect(closed.DupLiveMembers()); !slices.Equal(got, []closed.Dup{closed.DupA, closed.DupC}) {
		t.Errorf("DupLiveMembers = %v, want the declarations not retired", got)
	}
	if got := slices.Collect(closed.CodedLiveMembers()); !slices.Equal(got, []closed.Coded{closed.CodedQ, closed.CodedR}) {
		t.Errorf("CodedLiveMembers = %v", got)
	}
}
`

const retiredOpenSmoke = `package open_test

import (
	"slices"
	"testing"

	open "example.com/retired/open"
)

func TestRetired(t *testing.T) {
	if open.OpenedX.Retired() || !open.OpenedY.Retired() || open.Opened("zzz").Retired() || open.Opened("").Retired() {
		t.Error("an open enum's Retired is wrong")
	}
	if got := slices.Collect(open.OpenDupLiveMembers()); !slices.Equal(got, []open.OpenDup{open.OpenDupFirst, open.OpenDupThird, open.OpenDupFifth}) {
		t.Errorf("OpenDupLiveMembers = %v, want [first third fifth]", got)
	}
	if got := slices.Collect(open.OpenedLiveMembers()); !slices.Equal(got, []open.Opened{open.OpenedX}) {
		t.Errorf("OpenedLiveMembers = %v", got)
	}
}
`

// CODEGEN.md §5.2, §5.3, DECISIONS 395: Retired reads a flag string by position (by code with @codes, by wire when open, by entry for an id), a value outside the members is not retired, and <E>LiveMembers skips the retired members.
func TestRetiredAndLiveMembers(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	_, files := buildWorld(t, retiredFixture())
	for _, c := range []struct{ path, want string }{{retiredClosed, "func (self ItemID) Retired() bool {"}, {retiredOpen, "func OpenedLiveMembers() iter.Seq[Opened] {"}} {
		if !strings.Contains(string(files[c.path]), c.want) {
			t.Errorf("%s has no %q", c.path, c.want)
		}
	}
	mod := t.TempDir()
	writeNoncedModule(t, mod, retiredModule, files, map[string][]byte{"closed/smoke_test.go": []byte(retiredClosedSmoke), "open/smoke_test.go": []byte(retiredOpenSmoke)})
	goCmd(t, mod, "vet", "./...")
	goCmd(t, mod, "test", "-count=1", "./...")
}
