package cppgen_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	bigEnumMembers = 60000
	bigEnumPkg     = "big.enum"
	syntaxOnly     = "-fsyntax-only"
)

// bigEnumPackage is a baked package with an enum and a table of bigEnumMembers entries each, whose
// names would take clang past its constant-evaluation step limit as plain literals.
func bigEnumPackage() *ir.Package {
	members := make([]*ir.EnumMember, bigEnumMembers)
	ids := make([]string, bigEnumMembers)
	rec := &ir.Record{Pkg: bigEnumPkg, Name: "Slot", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	rt := bulkTypeOf(bigEnumPkg, "Slot", rec)
	coll := &types.Collection{Kind: types.CollLet, Pkg: bigEnumPkg, Name: "slots"}
	rows := make([]*value.Record, bigEnumMembers)
	for i := range members {
		name := fmt.Sprintf("BIG_ENUMERATED_MEMBER_NUMBER_%05d", i)
		members[i] = &ir.EnumMember{Name: name, Wire: name, Index: i}
		ids[i] = fmt.Sprintf("slot_with_a_long_identifier_%05d", i)
		rows[i] = &value.Record{T: rt, Fields: []value.Value{num(int64(i))}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: ids[i]}}}
	}
	elem := ir.TypeRef{Kind: types.Record, Named: rec}
	return &ir.Package{
		Name: bigEnumPkg, Dir: "big/enum",
		Types:  []ir.Type{&ir.Enum{Pkg: bigEnumPkg, Name: "Big", Members: members}, rec},
		Values: []*ir.Value{{Name: "slots", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, V: &value.Table{Entries: rows}}},
		Emits:  []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "big/enum/out", Mode: ir.ModeBaked, Namespace: "big::enumx"}},
	}
}

// DECISIONS 392, CODEGEN.md §5.2, §5.3: the ToName, ToWire and FromWire tables of a 60 000-member enum and id enum pass every compiler's constant evaluation.
func TestBigEnumTablesCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("checks a source of two 60 000-entry enums")
	}
	t.Parallel()
	checkSyntax(t, generate(t, bigEnumPackage()), "enum.gen.cpp")
}

// checkSyntax compiles source of the generated files with -fsyntax-only on every compiler.
func checkSyntax(t *testing.T, files []ir.File, source string) {
	t.Helper()
	compilers := cxx.Compilers(t)
	dir := t.TempDir()
	writeFiles(t, dir, files)
	outs, errs := make([][]byte, len(compilers)), make([]error, len(compilers))
	var wg sync.WaitGroup
	for i, cc := range compilers {
		args := append(append([]string(nil), cxx.Flags...), syntaxOnly, "-I", dir, filepath.Join(dir, source))
		wg.Go(func() { outs[i], errs[i] = compile(cc, args, dir) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("%s: %v\n%.2000s", filepath.Base(compilers[i]), err, outs[i])
		}
	}
}

const (
	bigFindRows  = 60000
	bigFindPkg   = "big.find"
	bigFindWidth = 31 // "find_key_" and 31 digits: a 40-byte key
)

// bigFindPackage is a baked package with a keyed list of bigFindRows rows, 40-byte String keys, in descending order.
func bigFindPackage() *ir.Package {
	rec := &ir.Record{Pkg: bigFindPkg, Name: "Slot", Fields: []*ir.Field{field("name", "name", "", tString)}}
	rt := bulkTypeOf(bigFindPkg, "Slot", rec)
	coll := &types.Collection{Kind: types.CollLet, Pkg: bigFindPkg, Name: "slots"}
	rows := make([]*value.Record, bigFindRows)
	for i := range rows {
		name := fmt.Sprintf("find_key_%0*d", bigFindWidth, bigFindRows-i)
		rows[i] = &value.Record{T: rt, Fields: []value.Value{str(name)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: name}}}
	}
	return &ir.Package{
		Name: bigFindPkg, Dir: "big/find", Types: []ir.Type{rec}, Values: []*ir.Value{findList("slots", rec, "name", rows...)},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "big/find/out", Mode: ir.ModeBaked, Namespace: "big::find"}},
	}
}

// CODEGEN.md §5.9, DECISIONS 392, 432: the sorted key table of a 60 000-row keyed list with 40-byte String keys passes every compiler's constant evaluation (sized views, as an enum's tables).
func TestBigFindTablesCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("checks a source of one 60 000-key table")
	}
	t.Parallel()
	checkSyntax(t, generate(t, bigFindPackage()), "find.gen.cpp")
}
