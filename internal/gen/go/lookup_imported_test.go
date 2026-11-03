package gogen_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// The imported enums of TestLookupImportedEnums (CODEGEN.md §5.10, DECISIONS 435): lookups of one package over another package's closed enums, dense and sparse out of declaration order, and over an enum that package opens, whose values are its wires.
const (
	importedUserFile = "user/user.gen.go"
	importedSparse   = "Sparse"
	importedDense    = "Dense"
	importedOpen     = "Open"
	importedWireFmt  = "w%04d"
	importedNoMember = "nope"
)

// importedLookups are the lookups of the user package: the function, the enum, the package the enum is in, the enum's codes, and its member's code at declaration index i (-1 for an opened enum, whose members are told by wire).
var importedLookups = []struct {
	fn, enum, pkg string
	code          func(i int) int64
}{
	{"WeightSparse", importedSparse, "shared", func(i int) int64 { return int64((i*lookupStride)%lookupMembers*lookupCodeStep + 1) }},
	{"WeightDense", importedDense, "shared", func(i int) int64 { return int64(denseFirst + i) }},
	{"WeightOpen", importedOpen, "open", func(i int) int64 { return int64((i*lookupStride)%lookupMembers*lookupCodeStep + 1) }},
}

// importedFixture is three packages: shared (baked) holds the closed enums, open (types) opens its enum, and user (baked) imports both and holds a lookup over each enum, n members each; the cell of member i is i times cellStep.
func importedFixture(n int) *fixture {
	shared := &pkgSpec{
		Name: "reads.shared", Dir: "reads/shared",
		Emit: emitSpec{Out: "out/", Dir: "reads/shared", Import: readsModule + "/shared", Package: "shared", Mode: "baked"},
	}
	open := &pkgSpec{
		Name: "reads.open", Dir: "reads/open",
		Emit: emitSpec{Out: "out/", Dir: "reads/open", Import: readsModule + "/open", Package: "open", Mode: "types", Open: []string{importedOpen}},
	}
	user := &pkgSpec{
		Name: "reads.user", Dir: "reads/user", Imports: []string{shared.Name, open.Name},
		Emit: emitSpec{Out: "out/", Dir: "reads/user", Import: readsModule + "/user", Package: "user", Mode: "baked"},
	}
	owners := map[string]*pkgSpec{"shared": shared, "open": open}
	for _, c := range importedLookups {
		spec := enumSpec{Name: c.enum, Codes: "UInt16"}
		fn := fnSpec{Name: c.fn, Result: "Int", Params: []paramSpec{{Name: "e", Type: c.enum}}}
		for i := range n {
			member := fmt.Sprintf("m%04d", i)
			if c.enum == importedOpen {
				member += "=" + fmt.Sprintf(importedWireFmt, (i*lookupStride)%n)
			}
			spec.Members = append(spec.Members, member)
			spec.CodeValues = append(spec.CodeValues, c.code(i))
			fn.Cells = append(fn.Cells, json.RawMessage(strconv.Itoa(i*cellStep)))
		}
		owners[c.pkg].Enums = append(owners[c.pkg].Enums, spec)
		user.Fns = append(user.Fns, fn)
	}
	return &fixture{Module: readsModule, Packages: []*pkgSpec{shared, open, user}}
}

// importedSmoke is a test of package user: every member of each imported enum answers its cell, and a value that is no member panics as index -1 always did.
func importedSmoke(n int) []byte {
	var b strings.Builder
	b.WriteString("package user_test\n\nimport (\n\t\"testing\"\n\n\topen \"example.com/reads/open\"\n\tshared \"example.com/reads/shared\"\n\tu \"example.com/reads/user\"\n)\n\n")
	b.WriteString("func panics(f func()) (p bool) {\n\tdefer func() { p = recover() != nil }()\n\tf()\n\treturn false\n}\n\n")
	b.WriteString("func TestImported(t *testing.T) {\n")
	for _, c := range importedLookups {
		for i := range n {
			fmt.Fprintf(&b, "\tif got := u.%s(%s.%sM%04d); got != %d {\n\t\tt.Errorf(\"%s(M%04d) = %%d\", got)\n\t}\n", c.fn, c.pkg, c.enum, i, i*cellStep, c.fn, i)
		}
		none := "0"
		if c.pkg == "open" {
			none = strconv.Quote(importedNoMember)
		}
		fmt.Fprintf(&b, "\tif !panics(func() { u.%s(%s.%s(%s)) }) {\n\t\tt.Errorf(\"%s of a value that is no member answered\")\n\t}\n", c.fn, c.pkg, c.enum, none, c.fn)
	}
	b.WriteString("}\n")
	return []byte(b.String())
}

// CODEGEN.md §5.10, DECISIONS 435: lookups over another package's enums, closed dense, closed sparse out of declaration order, and opened (searched by wire), answer as the switches did, each member its cell and a non-member the failure of index -1, from tables of the lookup's own package; the functions have the same size at 10 and at 500 members.
func TestLookupImportedEnums(t *testing.T) {
	sizes := func(n int) map[string]int {
		_, files := buildWorld(t, importedFixture(n))
		out := map[string]int{}
		for _, c := range importedLookups {
			out[c.fn] = partFuncs(t, files[importedUserFile])[c.fn].Body
		}
		return out
	}
	few, many := sizes(fewMembers2), sizes(lookupMembers*lookupMembers)
	for _, c := range importedLookups {
		if few[c.fn] == 0 || few[c.fn] != many[c.fn] {
			t.Errorf("%s has %d lines at %d members and %d at %d", c.fn, few[c.fn], fewMembers2, many[c.fn], lookupMembers*lookupMembers)
		}
	}
	_, files := buildWorld(t, importedFixture(lookupMembers))
	src := string(files[importedUserFile])
	for _, want := range []string{"var lookupKeys_reads_shared_Sparse = ", "var lookupAt_reads_shared_Sparse = ", "var lookupKeys_reads_open_Open = "} {
		if !strings.Contains(src, want) {
			t.Errorf("user.gen.go has no %q", want)
		}
	}
	if strings.Contains(src, "reads_shared_Dense") {
		t.Errorf("a dense closed enum has lookup tables")
	}
	mod := t.TempDir()
	writeNoncedModule(t, mod, readsModule, files, map[string][]byte{"user/smoke_test.go": importedSmoke(lookupMembers)})
	goCmd(t, mod, "vet", "./...")
	goCmd(t, mod, "test", "-count=1", "./...")
}
