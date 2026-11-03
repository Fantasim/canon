package gogen_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// The enums of the input and lookup tests (CODEGEN.md §2.11, §5.10, §5.12, DECISIONS 435): an input enum of n members, and lookups over enums whose codes are dense, dense from a negative first, and sparse out of declaration order.
const (
	readsModule    = "example.com/reads"
	inputPkg       = "readinput"
	inputFile      = "readinput/out/go/readinput.gen.go"
	readInputEnv   = "READINPUT_MODE"
	loadInputsFn   = "LoadInputs"
	fewMembers2    = 10
	manyInputs     = 2000
	lookupFile     = "lookup/lookup.gen.go"
	lookupMembers  = 12
	lookupStride   = 5 // coprime with lookupMembers: a permutation
	lookupCodeStep = 3
	negativeFirst  = -3
	denseFirst     = 100
	cellStep       = 7
	noMemberNeg    = 100
	noMemberSparse = 2
)

// lookupEnums are the three enums a lookup indexes, with the function that reads each: the function, the enum, its codes' type, its member's code at declaration index i, and a code no member has.
var lookupEnums = []struct {
	fn, enum, codes string
	code            func(i int) int64
	none            int
}{
	{"WeightDense", "Dense", "UInt16", func(i int) int64 { return int64(denseFirst + i) }, 0},
	{"WeightNeg", "Neg", "Int8", func(i int) int64 { return int64(negativeFirst + i) }, noMemberNeg},
	{"WeightSparse", "Sparse", "UInt16", func(i int) int64 { return int64((i*lookupStride)%lookupMembers*lookupCodeStep + 1) }, noMemberSparse},
}

// inputEnumPkg is a baked package with one record whose optional enum field is read from the environment: n members, wires w0000…, the third retired, the fifth sharing the fourth's wire.
func inputEnumPkg(n int) *ir.Package {
	e := &ir.Enum{Pkg: inputPkg, Name: "Mode"}
	for i := range n {
		wire := fmt.Sprintf("w%04d", i)
		if i == 4 {
			wire = fmt.Sprintf("w%04d", 3)
		}
		e.Members = append(e.Members, &ir.EnumMember{Name: fmt.Sprintf("m%04d", i), Wire: wire, Index: i, Retired: i == 2})
	}
	rec := &ir.Record{Pkg: inputPkg, Name: "Gen", Fields: []*ir.Field{
		{Name: "mode", Type: ir.TypeRef{Kind: types.Enum, Named: e}, Optional: true, Input: &types.Input{Env: readInputEnv}},
	}}
	return &ir.Package{
		Name: inputPkg, Dir: inputPkg, Types: []ir.Type{e, rec},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: inputPkg + "/out/go",
			GoImport: dataModule + "/" + inputPkg + "/out/go", Mode: ir.ModeBaked, GoPackage: inputPkg,
		}},
	}
}

// CODEGEN.md §2.11, §5.12, DECISIONS 435, EVALUATION.md §11.3: an enum input reads the enum's tables through Parse<E>, so LoadInputs has the same size at 10 and at 2,000 members and is within the budget at 5,000.
func TestInputEnumUsesTables(t *testing.T) {
	size := func(n int) int {
		return partFuncs(t, generateData(t, inputEnumPkg(n))[inputFile])[loadInputsFn].Body
	}
	few, many := size(fewMembers2), size(manyInputs)
	if few != many {
		t.Errorf("LoadInputs has %d lines at %d members and %d at %d", few, fewMembers2, many, manyInputs)
	}
	if got := size(manyInputs * 3); got > ir.PartLines {
		t.Errorf("LoadInputs has %d lines at %d members, over the budget of %d", got, manyInputs*3, ir.PartLines)
	}
}

// lookupFixture is a baked package of the three enums of lookupEnums, n members each, and a lookup over each: the cell of member i is i times cellStep.
func lookupFixture(n int) *fixture {
	pkg := &pkgSpec{
		Name: "reads.lookup", Dir: "reads/lookup",
		Emit: emitSpec{Out: "out/", Dir: "reads/lookup", Import: readsModule + "/lookup", Package: "lookup", Mode: "baked"},
	}
	for _, c := range lookupEnums {
		spec := enumSpec{Name: c.enum, Codes: c.codes}
		fn := fnSpec{Name: c.fn, Result: "Int", Params: []paramSpec{{Name: "e", Type: c.enum}}}
		for i := range n {
			spec.Members = append(spec.Members, fmt.Sprintf("m%04d", i))
			spec.CodeValues = append(spec.CodeValues, c.code(i))
			fn.Cells = append(fn.Cells, json.RawMessage(strconv.Itoa(i*cellStep)))
		}
		pkg.Enums, pkg.Fns = append(pkg.Enums, spec), append(pkg.Fns, fn)
	}
	return &fixture{Module: readsModule, Packages: []*pkgSpec{pkg}}
}

// CODEGEN.md §2.11, §5.10, DECISIONS 435: a lookup over a @codes enum indexes through the enum's ordinal (the code minus the first when dense, a search of the enum's members otherwise), so its function has the same size at 10 and at 2,000 members.
func TestLookupIndexDoesNotGrow(t *testing.T) {
	sizes := func(n int) map[string]int {
		_, files := buildWorld(t, lookupFixture(n))
		out := map[string]int{}
		for _, c := range lookupEnums {
			out[c.fn] = partFuncs(t, files[lookupFile])[c.fn].Body
		}
		return out
	}
	few, many := sizes(fewMembers2), sizes(manyInputs)
	for _, c := range lookupEnums {
		if few[c.fn] == 0 || few[c.fn] != many[c.fn] {
			t.Errorf("%s has %d lines at %d members and %d at %d", c.fn, few[c.fn], fewMembers2, many[c.fn], manyInputs)
		}
	}
}

// lookupSmoke is a test of package lookup: every member answers its cell, and a value that is no member panics as the table's index -1 always did.
func lookupSmoke(n int) []byte {
	var b strings.Builder
	b.WriteString("package lookup_test\n\nimport (\n\t\"testing\"\n\n\tl \"example.com/reads/lookup\"\n)\n\n")
	b.WriteString("func panics(f func()) (p bool) {\n\tdefer func() { p = recover() != nil }()\n\tf()\n\treturn false\n}\n\n")
	b.WriteString("func TestLookups(t *testing.T) {\n")
	for _, c := range lookupEnums {
		for i := range n {
			fmt.Fprintf(&b, "\tif got := l.%s(l.%sM%04d); got != %d {\n\t\tt.Errorf(\"%s(M%04d) = %%d\", got)\n\t}\n", c.fn, c.enum, i, i*cellStep, c.fn, i)
		}
		fmt.Fprintf(&b, "\tif !panics(func() { l.%s(l.%s(%d)) }) {\n\t\tt.Errorf(\"%s of a value that is no member answered\")\n\t}\n", c.fn, c.enum, c.none, c.fn)
	}
	b.WriteString("}\n")
	return []byte(b.String())
}

// CODEGEN.md §5.10, DECISIONS 435: the lookups answer as the switches did: each member its cell (dense, dense from a negative first, sparse out of declaration order), a non-member the failure of index -1.
func TestLookupIndexRuns(t *testing.T) {
	_, files := buildWorld(t, lookupFixture(lookupMembers))
	mod := t.TempDir()
	writeNoncedModule(t, mod, readsModule, files, map[string][]byte{"lookup/smoke_test.go": lookupSmoke(lookupMembers)})
	goCmd(t, mod, "vet", "./...")
	goCmd(t, mod, "test", "-count=1", "./...")
}

// inputSmoke is a test of package readinput: each live wire parses to its first holder, the retired member's wire and a stranger's are refused.
func inputSmoke() []byte {
	return []byte(`package readinput_test

import (
	"strings"
	"testing"

	p "example.com/data/readinput/out/go"
)

func TestInputs(t *testing.T) {
	for _, tc := range []struct {
		wire string
		want p.Mode
	}{{"w0000", p.ModeM0000}, {"w0001", p.ModeM0001}, {"w0003", p.ModeM0003}, {"w0005", p.ModeM0005}} {
		t.Setenv("` + readInputEnv + `", tc.wire)
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs(%q): %v", tc.wire, err)
		}
		if got, ok := (&p.Gen{}).Mode(); !ok || got != tc.want {
			t.Errorf("Mode() on %q = %v, %v, want %v", tc.wire, got, ok, tc.want)
		}
	}
	for _, wire := range []string{"w0002", "w0004", "nope"} {
		t.Setenv("` + readInputEnv + `", wire)
		err := p.LoadInputs()
		if err == nil || !strings.Contains(err.Error(), "` + readInputEnv + `: not a member of Mode") {
			t.Errorf("LoadInputs(%q) = %v, want a refusal", wire, err)
		}
	}
}
`)
}

// EVALUATION.md §11.3, DECISIONS 383, 435: reading the enum through Parse<E> answers as the switch did: the first holder of a shared wire, a refused retired member, a refused stranger.
func TestInputEnumReads(t *testing.T) {
	files := generateData(t, inputEnumPkg(lookupMembers))
	dir := writeModule(t, files, inputPkg+"/out/go", "", nil)
	if err := os.WriteFile(filepath.Join(dir, inputPkg, "out", "go", "smoke_test.go"), inputSmoke(), 0o644); err != nil {
		t.Fatal(err)
	}
	goCmd(t, dir, "vet", "./...")
	goCmd(t, dir, "test", "-count=1", "./...")
}
