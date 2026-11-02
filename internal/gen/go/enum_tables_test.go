package gogen_test

import (
	"strings"
	"testing"
)

// The enums of TestDenseCodesIndex and TestOpenEnumTablesOnce: dense @codes (from 6000, from the type's minimum, at the top of int64), sparse @codes declared out of order, an opened enum of each kind.
const (
	tablesModule = "example.com/tables"
	tablesClosed = "closed/closed.gen.go"
	tablesOpen   = "open/open.gen.go"
)

// tablesFixture has a baked package of closed enums (D, N and M dense, S sparse, W without codes and with a wire per name) and a types package opening four of them (OD dense, OS sparse, ON with every wire its name, OW with a wire that is not).
func tablesFixture() *fixture {
	enum := func(name string, members []string, codes string, values []int64, retired ...string) enumSpec {
		return enumSpec{Name: name, Members: members, Codes: codes, CodeValues: values, Retired: retired}
	}
	const top = int64(1<<63 - 1)
	return &fixture{Module: tablesModule, Packages: []*pkgSpec{
		{
			Name: "tab.closed", Dir: "tab/closed",
			Enums: []enumSpec{
				enum("D", []string{"a", "b", "c"}, "UInt16", []int64{6000, 6001, 6002}, "b"),
				enum("N", []string{"p", "q=wq"}, "Int8", []int64{-128, -127}),
				enum("M", []string{"lo", "hi"}, "Int", []int64{top - 1, top}),
				enum("S", []string{"x", "y", "z"}, "UInt8", []int64{9, 1, 4}, "x"),
				enum("W", []string{"u=wu", "v"}, "", nil),
			},
			Emit: emitSpec{Out: "out/", Dir: "tab/closed", Import: tablesModule + "/closed", Package: "closed", Mode: "baked"},
		},
		{
			Name: "tab.open", Dir: "tab/open",
			Enums: []enumSpec{
				enum("OD", []string{"a", "b=wb", "c"}, "UInt8", []int64{5, 6, 7}),
				enum("OS", []string{"m", "n", "o"}, "UInt8", []int64{9, 2, 4}, "n"),
				enum("ON", []string{"r", "s"}, "", nil),
				enum("OW", []string{"t=wt", "u"}, "", nil),
			},
			Emit: emitSpec{Out: "out/", Dir: "tab/open", Import: tablesModule + "/open", Package: "open", Mode: "types", Open: []string{"OD", "OS", "ON", "OW"}},
		},
	}}
}

const tablesClosedSmoke = `package closed_test

import (
	"math"
	"strings"
	"testing"

	closed "example.com/tables/closed"
)

func TestDense(t *testing.T) {
	for _, c := range []struct {
		code uint16
		want string
	}{{6000, "a"}, {6001, "b"}, {6002, "c"}, {5999, "D(5999)"}, {6003, "D(6003)"}, {0, "D(0)"}, {math.MaxUint16, "D(65535)"}} {
		if got := closed.D(c.code).String(); got != c.want {
			t.Errorf("D(%d).String() = %q, want %q", c.code, got, c.want)
		}
		got, ok := closed.DFromCode(c.code)
		if ok != !strings.HasPrefix(c.want, "D(") || ok && got != closed.D(c.code) {
			t.Errorf("DFromCode(%d) = %v, %v", c.code, got, ok)
		}
	}
	if !closed.DB.Retired() || closed.DA.Retired() || closed.D(6003).Retired() || closed.D(5999).Retired() {
		t.Error("D.Retired")
	}
	if got, ok := closed.ParseD("c"); !ok || got != closed.DC {
		t.Errorf("ParseD(c) = %v, %v", got, ok)
	}
	if _, ok := closed.ParseD("d"); ok {
		t.Error("ParseD(d) found a member")
	}
	for _, v := range []closed.D{closed.DA, closed.DB, closed.DC} {
		if got, ok := closed.ParseD(v.Wire()); !ok || got != v {
			t.Errorf("ParseD(%q) = %v, %v", v.Wire(), got, ok)
		}
	}
}

func TestDenseNegative(t *testing.T) {
	if closed.NP.String() != "p" || closed.NQ.String() != "q" || closed.NQ.Wire() != "wq" || closed.N(-126).Wire() != "N(-126)" || closed.N(127).String() != "N(127)" {
		t.Errorf("N: %q %q %q %q %q", closed.NP.String(), closed.NQ.String(), closed.NQ.Wire(), closed.N(-126).Wire(), closed.N(127).String())
	}
	for code, want := range map[int8]bool{-128: true, -127: true, -126: false, 127: false, 0: false} {
		if _, ok := closed.NFromCode(code); ok != want {
			t.Errorf("NFromCode(%d) ok = %v", code, ok)
		}
	}
	if got, ok := closed.ParseN("wq"); !ok || got != closed.NQ {
		t.Errorf("ParseN(wq) = %v, %v", got, ok)
	}
	if _, ok := closed.ParseN("q"); ok {
		t.Error("ParseN(q) found a member by its name")
	}
}

func TestDenseTop(t *testing.T) {
	for code, want := range map[int64]bool{math.MaxInt64 - 1: true, math.MaxInt64: true, math.MaxInt64 - 2: false, 0: false, math.MinInt64: false} {
		if _, ok := closed.MFromCode(code); ok != want {
			t.Errorf("MFromCode(%d) ok = %v", code, ok)
		}
	}
	if closed.MHi.String() != "hi" || closed.MLo.String() != "lo" || closed.M(math.MinInt64).String() == "lo" || closed.M(math.MaxInt64-2).Wire() == "lo" {
		t.Error("M names")
	}
	if got, ok := closed.ParseM("hi"); !ok || got != closed.MHi {
		t.Errorf("ParseM(hi) = %v, %v", got, ok)
	}
}

func TestSparse(t *testing.T) {
	for code, want := range map[uint8]string{9: "x", 1: "y", 4: "z", 0: "S(0)", 5: "S(5)", 10: "S(10)"} {
		if got := closed.S(code).String(); got != want {
			t.Errorf("S(%d).String() = %q, want %q", code, got, want)
		}
	}
	if got, ok := closed.SFromCode(4); !ok || got != closed.SZ {
		t.Errorf("SFromCode(4) = %v, %v", got, ok)
	}
	if _, ok := closed.SFromCode(5); ok {
		t.Error("SFromCode(5) found a member")
	}
	if !closed.SX.Retired() || closed.SY.Retired() || closed.S(5).Retired() {
		t.Error("S.Retired")
	}
	for _, v := range []closed.S{closed.SX, closed.SY, closed.SZ} {
		if got, ok := closed.ParseS(v.Wire()); !ok || got != v {
			t.Errorf("ParseS(%q) = %v, %v", v.Wire(), got, ok)
		}
	}
	if _, ok := closed.ParseS("w"); ok {
		t.Error("ParseS(w) found a member")
	}
}

func TestOpaque(t *testing.T) {
	if got, ok := closed.ParseW("wu"); !ok || got != closed.WU {
		t.Errorf("ParseW(wu) = %v, %v", got, ok)
	}
	if got, ok := closed.ParseW("v"); !ok || got != closed.WV {
		t.Errorf("ParseW(v) = %v, %v", got, ok)
	}
	if _, ok := closed.ParseW("u"); ok {
		t.Error("ParseW(u) found a member by its name")
	}
	if closed.WU.String() != "u" || closed.WU.Wire() != "wu" {
		t.Errorf("WU: %q %q", closed.WU.String(), closed.WU.Wire())
	}
}
`

const tablesOpenSmoke = `package open_test

import (
	"slices"
	"testing"

	open "example.com/tables/open"
)

func TestOpenDense(t *testing.T) {
	if !slices.Equal(slices.Collect(open.ODMembers()), []open.OD{open.ODA, open.ODB, open.ODC}) {
		t.Errorf("ODMembers = %v", slices.Collect(open.ODMembers()))
	}
	if code, ok := open.ODB.Code(); !ok || code != 6 {
		t.Errorf("ODB.Code() = %d, %v", code, ok)
	}
	for code, want := range map[uint8]open.OD{5: open.ODA, 6: open.ODB, 7: open.ODC} {
		if got, ok := open.ODFromCode(code); !ok || got != want {
			t.Errorf("ODFromCode(%d) = %q, %v", code, got, ok)
		}
	}
	for _, code := range []uint8{0, 4, 8, 255} {
		if got, ok := open.ODFromCode(code); ok || got != "" {
			t.Errorf("ODFromCode(%d) = %q, %v", code, got, ok)
		}
	}
	if open.ODB.String() != "b" || open.ODB.Wire() != "wb" || open.OD("zz").String() != "zz" || open.ODA.String() != "a" {
		t.Errorf("OD: %q %q %q", open.ODB.String(), open.ODB.Wire(), open.OD("zz").String())
	}
}

func TestOpenSparse(t *testing.T) {
	if !slices.Equal(slices.Collect(open.OSMembers()), []open.OS{open.OSM, open.OSN, open.OSO}) {
		t.Errorf("OSMembers = %v", slices.Collect(open.OSMembers()))
	}
	if !slices.Equal(slices.Collect(open.OSLiveMembers()), []open.OS{open.OSM, open.OSO}) {
		t.Errorf("OSLiveMembers = %v", slices.Collect(open.OSLiveMembers()))
	}
	for code, want := range map[uint8]open.OS{9: open.OSM, 2: open.OSN, 4: open.OSO} {
		if got, ok := open.OSFromCode(code); !ok || got != want {
			t.Errorf("OSFromCode(%d) = %q, %v", code, got, ok)
		}
	}
	if _, ok := open.OSFromCode(3); ok {
		t.Error("OSFromCode(3) found a member")
	}
	if code, ok := open.OSO.Code(); !ok || code != 4 {
		t.Errorf("OSO.Code() = %d, %v", code, ok)
	}
	if !open.OSN.Retired() || open.OSM.Retired() || open.OS("q").Retired() {
		t.Error("OS.Retired")
	}
}

func TestOpenNames(t *testing.T) {
	if open.ONR.String() != "r" || open.ONS.String() != "s" || open.ON("zz").String() != "zz" || !open.ONR.Known() || open.ON("zz").Known() {
		t.Errorf("ON: %q %q %q", open.ONR.String(), open.ONS.String(), open.ON("zz").String())
	}
	if open.OWT.String() != "t" || open.OWT.Wire() != "wt" || open.OWU.String() != "u" || open.OW("zz").String() != "zz" || open.OW("t").Known() {
		t.Errorf("OW: %q %q %q %q", open.OWT.String(), open.OWT.Wire(), open.OWU.String(), open.OW("zz").String())
	}
	if !slices.Equal(slices.Collect(open.OWMembers()), []open.OW{open.OWT, open.OWU}) || !slices.Equal(slices.Collect(open.ONMembers()), []open.ON{open.ONR, open.ONS}) {
		t.Error("members")
	}
	if got, ok := open.ParseOW("wt"); !ok || got != open.OWT {
		t.Errorf("ParseOW(wt) = %q, %v", got, ok)
	}
}
`

// CODEGEN.md §5.2, DECISIONS 430: a closed @codes enum whose codes are dense in declaration order has no sorted-codes table, String, Wire, Retired and <E>FromCode index by the code minus the first, in uint64 (from 6000, from the type's minimum, at the top of int64); a sparse one keeps its sorted codes; each answers as before on members and non-members.
func TestDenseCodesIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	_, files := buildWorld(t, tablesFixture())
	closed := string(files[tablesClosed])
	for _, name := range []string{"D", "N", "M"} {
		if table := "enumCodes_" + name + " "; strings.Contains(closed, table) {
			t.Errorf("the dense %s has %s", name, table)
		}
	}
	for _, want := range []string{"enumCodes_S =", "uint64(int64(self) - 6000)", "uint64(int64(self) - (-128))", "uint64(int64(code)-6000)"} {
		if !strings.Contains(closed, want) {
			t.Errorf("the closed file has no %q", want)
		}
	}
	mod := t.TempDir()
	writeNoncedModule(t, mod, tablesModule, files, map[string][]byte{"closed/smoke_test.go": []byte(tablesClosedSmoke), "open/smoke_test.go": []byte(tablesOpenSmoke)})
	goCmd(t, mod, "vet", "./...")
	goCmd(t, mod, "test", "-count=1", "./closed/...")
}

// CODEGEN.md §5.2, DECISIONS 430: a closed enum searches enumByWire_<T>, the indexes of the names' order sorted by wire, and keeps neither enumKeys_ nor enumFound_.
func TestParseSearchesByWire(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a package")
	}
	_, files := buildWorld(t, tablesFixture())
	closed := string(files[tablesClosed])
	for _, name := range []string{"D", "N", "M", "S", "W"} {
		if want := "var enumByWire_" + name + " = [...]uint8{"; !strings.Contains(closed, want) {
			t.Errorf("the closed file has no %q", want)
		}
	}
	for _, unwanted := range []string{"enumKeys_", "enumFound_"} {
		if strings.Contains(closed, unwanted) {
			t.Errorf("the closed file holds %s", unwanted)
		}
	}
	if !strings.Contains(closed, `"strings"`) {
		t.Error("the closed file does not import strings")
	}
}

// CODEGEN.md §5.2, DECISIONS 430: an opened enum's one member table is enumKeys_<T>; enumMembers_<T> and, with codes that are not dense, enumByCode_<T> are uintN indexes into it; enumNames_<T> exists only when a name differs from its wire, and String returns the wire otherwise; a dense opened enum has no sorted-codes table; the answers are unchanged.
func TestOpenEnumTablesOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	_, files := buildWorld(t, tablesFixture())
	open := string(files[tablesOpen])
	for _, want := range []string{
		"var enumKeys_OD = [...]OD{", "var enumMembers_OD = [...]uint8{", "var enumNames_OD =", "var enumNumbers_OD =",
		"var enumMembers_OS = [...]uint8{", "var enumByCode_OS = [...]uint8{", "var enumCodes_OS = [...]uint8{",
		"var enumNames_OW =", "var enumMembers_ON = [...]uint8{",
	} {
		if !strings.Contains(open, want) {
			t.Errorf("the open file has no %q", want)
		}
	}
	for _, unwanted := range []string{"enumNames_ON", "enumByCode_OD", "enumCodes_OD ", "enumNames_OS", "enumMembers_OD = [...]OD", "enumByCode_OS = [...]OS"} {
		if strings.Contains(open, unwanted) {
			t.Errorf("the open file holds %q", unwanted)
		}
	}
	mod := t.TempDir()
	writeNoncedModule(t, mod, tablesModule, files, map[string][]byte{"closed/smoke_test.go": []byte(tablesClosedSmoke), "open/smoke_test.go": []byte(tablesOpenSmoke)})
	goCmd(t, mod, "test", "-count=1", "./open/...")
}
