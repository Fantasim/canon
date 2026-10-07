package edit_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// allOpBytes is how many bytes of FuzzMinimalWriteAll's input draw one operation after its
// first byte, which picks the project: three for the value it edits, one for the operation.
const (
	allOpBytes = 4
	floatUnits = 8     // a drawn Float is n eighths: exact in binary, with a fraction
	maxDurMs   = 1e12  // a drawn Duration stays far from time.Duration's overflow
	seedN      = 3     // the seeds' integer
	seedS      = "new" // the seeds' string and key
)

// On the benchmark an input draws one operation, and copies no whole table: Go's fuzz worker
// dies on an exec past 10 s, as a whole-table Set or a second operation may take (log-2026-09-29
// M4 P20).
const benchOps, benchCopies = 1, 1000

// projExamples and projBench name the projects in the -edit.tally file make fuzz-edit counts.
const projExamples, projBench = "examples", "bench"

// fuzzProject is a project the fuzz edits: every value of its lets and consts, and every
// member of its enums (API.md P7a), by positional path; an input draws ops operations at most,
// and a value it copies holds copies values at most (0: any).
type fuzzProject struct {
	env    edit.Env
	a      *build.Analysis
	cands  []candidate
	byPath map[string]int
	name   string
	ops    int
	copies int
}

// exampleRoots redirects every root of examples/project.canon: the loads to the fixtures, the
// emits to a directory of the test (examples/_fixtures/README.md).
func exampleRoots(t testing.TB) (string, map[string]string) {
	t.Helper()
	dir, err := filepath.Abs("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	out := t.TempDir()
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(out, name))
		if err := os.MkdirAll(filepath.Join(out, name), 0o750); err != nil { // SPEC §3.1: a required root exists
			t.Fatal(err)
		}
	}
	return filepath.ToSlash(dir), roots
}

// openFuzzProject analyzes every package of the project in dir and lists its values.
func openFuzzProject(t testing.TB, dir string, roots map[string]string) *fuzzProject {
	t.Helper()
	p, err := build.Open(project.OS(), dir, build.Options{Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	fz := &fuzzProject{env: edit.Env{Project: p, Host: hostOf}, a: a, byPath: map[string]int{}, name: projExamples, ops: maxOps}
	for _, pkg := range a.Program().Packages {
		for _, obj := range pkg.Decls {
			fz.cands = declValues(fz.cands, a, pkg.Path, obj)
		}
	}
	for i, c := range fz.cands {
		fz.byPath[c.path] = i
	}
	return fz
}

// declValues adds the values of a let or const, or the members of an enum, to out.
func declValues(out []candidate, a *build.Analysis, pkg string, obj check.Object) []candidate {
	root := pkg + ":" + obj.Name()
	switch obj.Kind() {
	case check.ObjLet, check.ObjConst:
		if v, ok := a.Force(eval.Root{Pkg: pkg, Name: obj.Name()}); ok {
			return collect(out, root, v)
		}
	case check.ObjTypeName:
		if e, ok := obj.Type().(*types.EnumType); ok {
			for i, m := range e.Members {
				out = append(out, candidate{path: root + "." + m.Name, v: &value.Member{Enum: e, Index: i}})
			}
		}
	}
	return out
}

// fuzzOp is an operation on a value drawn from d; false when the value takes none.
type fuzzOp func(fz *fuzzProject, c candidate, d draw) (edit.Operation, bool)

// allOps are every operation of the edit API, each drawn where it may apply: Set of a scalar, of
// none and of a sibling's value (API.md M1), Add, Insert, AddEntry, Remove, Move, Rename, Reset,
// Retire, Unretire and SetCase.
var allOps = []struct {
	kind edit.Op
	draw fuzzOp
}{
	{edit.OpSet, fzSet}, {edit.OpSet, fzSetNone}, {edit.OpSet, fzSetCopy}, {edit.OpAdd, fzAdd},
	{edit.OpInsert, fzInsert}, {edit.OpAddEntry, fzAddEntry}, {edit.OpRemove, fzRemove},
	{edit.OpMove, fzMove}, {edit.OpRename, fzRename}, {edit.OpReset, fzReset},
	{edit.OpRetire, fzRetire}, {edit.OpUnretire, fzUnretire}, {edit.OpSetCase, fzSetCase},
}

func fzSet(_ *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	lit, ok := anyScalar(c.v, d)
	return edit.Operation{Kind: edit.OpSet, Path: c.path, Value: lit}, ok
}

func fzSetNone(_ *fuzzProject, c candidate, _ draw) (edit.Operation, bool) {
	return edit.Operation{Kind: edit.OpSet, Path: c.path, Value: edit.None{}}, true
}

func fzSetCopy(fz *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	lit, ok := fz.lit(fz.sibling(c).v, d)
	return edit.Operation{Kind: edit.OpSet, Path: c.path, Value: lit}, ok
}

func fzAdd(fz *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	l, ok := c.v.(*value.List)
	if !ok || len(l.Elems) == 0 {
		return edit.Operation{}, false
	}
	lit, ok := fz.lit(l.Elems[0], d)
	return edit.Operation{Kind: edit.OpAdd, Path: c.path, Value: lit}, ok
}

func fzInsert(fz *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	op, ok := fzAdd(fz, c, d)
	if l, isList := c.v.(*value.List); ok && isList {
		op.Kind, op.Index = edit.OpInsert, int(uint64(d.n)%uint64(len(l.Elems)+1))
	}
	return op, ok
}

func fzAddEntry(fz *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	var key, lit edit.Lit
	ok := false
	switch x := c.v.(type) {
	case *value.Table:
		if len(x.Entries) > 0 && x.Entries[0].Ident != nil {
			key = drawnKey(x.Entries[0].Ident.Key.IsInt, d)
			lit, ok = fz.lit(x.Entries[0], d)
		}
	case *value.Map:
		if len(x.Keys) > 0 {
			key, ok = mapKey(x.Keys[0], d)
			lit, ok = fz.litIf(ok, x.Vals[0], d)
		}
	}
	return edit.Operation{Kind: edit.OpAddEntry, Path: c.path, Key: key, Value: lit}, ok
}

func fzRemove(_ *fuzzProject, c candidate, _ draw) (edit.Operation, bool) {
	return edit.Operation{Kind: edit.OpRemove, Path: c.path}, strings.HasSuffix(c.path, "]") || retirable(c)
}

func fzMove(_ *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	return edit.Operation{Kind: edit.OpMove, Path: c.path, Index: int(uint64(d.n) % moveSpan)}, strings.HasSuffix(c.path, "]")
}

func fzRename(_ *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	return edit.Operation{Kind: edit.OpRename, Path: c.path, Key: edit.Key(d.s)}, strings.HasSuffix(c.path, "]")
}

func fzReset(_ *fuzzProject, c candidate, _ draw) (edit.Operation, bool) {
	at := strings.LastIndexAny(c.path, ".]")
	return edit.Operation{Kind: edit.OpReset, Path: c.path}, at > strings.IndexByte(c.path, ':') && c.path[at] == '.'
}

func fzRetire(_ *fuzzProject, c candidate, _ draw) (edit.Operation, bool) {
	return edit.Operation{Kind: edit.OpRetire, Path: c.path}, retirable(c)
}

// fzUnretire draws the retired entries and members, which Unretire always refuses (API.md E4).
func fzUnretire(_ *fuzzProject, c candidate, _ draw) (edit.Operation, bool) {
	return edit.Operation{Kind: edit.OpUnretire, Path: c.path}, retired(c)
}

func fzSetCase(_ *fuzzProject, c candidate, d draw) (edit.Operation, bool) {
	r, ok := c.v.(*value.Record)
	if !ok {
		return edit.Operation{}, false
	}
	ct, ok := r.T.(*types.CaseType)
	if !ok {
		return edit.Operation{}, false
	}
	name := ct.Variant.Cases[int(uint64(d.n)%uint64(len(ct.Variant.Cases)))].Name
	return edit.Operation{Kind: edit.OpSetCase, Path: c.path, Case: name}, true
}

// retirable is an entry or keyed element, or an enum member named by its enum (API.md P7a).
func retirable(c candidate) bool {
	if r, ok := c.v.(*value.Record); ok {
		return r.Ident != nil
	}
	return enumMember(c)
}

// retired is a retired entry, or a retired member named by its enum.
func retired(c candidate) bool {
	if r, ok := c.v.(*value.Record); ok {
		return r.Ident != nil && r.Ident.Retired
	}
	m, ok := c.v.(*value.Member)
	return ok && enumMember(c) && m.Enum.Members[m.Index].Retired
}

// enumMember is whether c names an enum's member (API.md P7a) rather than a value.
func enumMember(c candidate) bool {
	m, ok := c.v.(*value.Member)
	return ok && strings.HasSuffix(c.path, ":"+m.Enum.Name+"."+m.Enum.Members[m.Index].Name)
}

// anyScalar is a value of a scalar's type drawn from d, durations and floats included.
func anyScalar(v value.Value, d draw) (edit.Lit, bool) {
	switch v.(type) {
	case *value.Float:
		return edit.Float(float64(d.n%maxDurMs) / floatUnits), true
	case *value.Dur:
		return edit.Dur(time.Duration(d.n%maxDurMs) * time.Millisecond), true
	}
	return scalar(v, d)
}

// drawnKey is a fresh key of a table: an integer or a word.
func drawnKey(isInt bool, d draw) edit.Lit {
	if isInt {
		return edit.IntKey(d.n)
	}
	return edit.Key(d.s)
}

// mapKey is a fresh key of the type of k, a key of the map (API.md P2).
func mapKey(k value.Value, d draw) (edit.Lit, bool) {
	switch x := k.(type) {
	case *value.Int:
		return edit.IntKey(d.n), true
	case *value.Member:
		return scalar(x, d)
	case *value.Str, *value.Ref:
		return edit.Key(d.s), true
	}
	return nil, false
}

// lit is v as an operation's value: a drawn scalar, else v's own literal, of at most fz.copies
// values when set.
func (fz *fuzzProject) lit(v value.Value, d draw) (edit.Lit, bool) {
	if lit, ok := anyScalar(v, d); ok {
		return lit, true
	}
	if fz.copies > 0 && rest(v, fz.copies) < 0 {
		return nil, false
	}
	lit, err := edit.SourceOf(context.Background(), fz.env, edit.NewSnapshot(fz.a), v)
	return lit, err == nil
}

// rest is n less the values v holds, itself included, counted until it falls below zero: every
// field, element, entry, map key and value, and pair side; a scalar or range counts one.
func rest(v value.Value, n int) int {
	n--
	switch x := v.(type) {
	case *value.Record:
		return restAll(x.Fields, n)
	case *value.List:
		return restAll(x.Elems, n)
	case *value.Map:
		return restAll(x.Vals, restAll(x.Keys, n))
	case *value.Pair:
		return restAll([]value.Value{x.A, x.B}, n)
	case *value.Table:
		for _, e := range x.Entries {
			if n < 0 {
				break
			}
			n = rest(e, n)
		}
	}
	return n
}

// restAll is rest over each of vs in turn, nil fields skipped.
func restAll(vs []value.Value, n int) int {
	for _, v := range vs {
		if n < 0 {
			return n
		}
		if v != nil {
			n = rest(v, n)
		}
	}
	return n
}

func (fz *fuzzProject) litIf(ok bool, v value.Value, d draw) (edit.Lit, bool) {
	if !ok {
		return nil, false
	}
	return fz.lit(v, d)
}

// sibling is the next (else the previous) element of the collection c lies in, at the same
// path below it; c itself when there is none: a Set of a value of the same type (API.md M1).
func (fz *fuzzProject) sibling(c candidate) candidate {
	at := strings.LastIndex(c.path, "[#")
	if at < 0 {
		return c
	}
	end := at + strings.IndexByte(c.path[at:], ']')
	i, err := strconv.Atoi(c.path[at+len("[#") : end])
	if err != nil {
		return c
	}
	for _, j := range []int{i + 1, i - 1} {
		if k, ok := fz.byPath[c.path[:at]+"[#"+strconv.Itoa(j)+c.path[end:]]; ok {
			return fz.cands[k]
		}
	}
	return c
}

// draw is the operations an input draws on fz, allOpBytes bytes each, fz.ops at most.
func (fz *fuzzProject) draw(raw []byte, n int64, s string) []edit.Operation {
	var ops []edit.Operation
	for i := 0; i+allOpBytes <= len(raw) && len(ops) < fz.ops; i += allOpBytes {
		at := int(raw[i])<<16 | int(raw[i+1])<<8 | int(raw[i+2])
		op := allOps[int(raw[i+3])%len(allOps)]
		if o, ok := op.draw(fz, fz.cands[at%len(fz.cands)], draw{n: n + int64(len(ops)), s: s}); ok {
			ops = append(ops, o)
		}
	}
	return ops
}

// fuzzSeed is one input of FuzzMinimalWriteAll: an operation on a value of pkg, which
// Editable says an edit can reach or not (never an enum member: not a value).
type fuzzSeed struct {
	raw      []byte
	pkg      string
	editable bool
}

// seeds are, for every package of fz and every operation, one input drawing the operation on
// the first value of the package Editable lets it edit, else on the first it may apply to;
// the project is the one first picks.
func (fz *fuzzProject) seeds(first byte) []fuzzSeed {
	snap := edit.NewSnapshot(fz.a)
	best := map[string]fuzzSeed{}
	var order []string
	for i, c := range fz.cands {
		pkg := c.path[:strings.IndexByte(c.path, ':')]
		for k, op := range allOps {
			key := pkg + ":" + strconv.Itoa(k)
			prev, seen := best[key]
			if seen && prev.editable {
				continue
			}
			if _, ok := op.draw(fz, c, draw{n: seedN, s: seedS}); !ok {
				continue
			}
			editable := !enumMember(c) && editableBy(snap, c.path, op.kind)
			if seen && !editable {
				continue
			}
			if !seen {
				order = append(order, key)
			}
			best[key] = fuzzSeed{raw: []byte{first, byte(i >> 16), byte(i >> 8), byte(i), byte(k)}, pkg: pkg, editable: editable}
		}
	}
	out := make([]fuzzSeed, len(order))
	for j, key := range order {
		out[j] = best[key]
	}
	return out
}

// editableBy is whether an operation of kind op can edit the value at path (API.md §7).
func editableBy(s *edit.Snapshot, path string, op edit.Op) bool {
	p, err := edit.Parse(path)
	if err != nil {
		return false
	}
	r, err := edit.Resolve(s, p)
	if err != nil {
		return false
	}
	e, err := s.Editable(r, op, "")
	return err == nil && e.Mode != edit.ModeNone
}

// API.md M6, E4 (M4 acceptance item 3): the seeds of FuzzMinimalWriteAll apply every operation
// but Unretire (always refused) and edit every example package holding an editable value.
func TestMinimalWriteAllOps(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	applied, pkgs := map[edit.Op]int{}, map[string]int{}
	for _, seed := range fz.seeds(0) {
		ops := fz.draw(seed.raw[1:], seedN, seedS)
		if seed.editable {
			pkgs[seed.pkg] += 0
		}
		if applyChecked(t, fz.env, fz.a, ops, false) {
			applied[ops[0].Kind]++
			pkgs[seed.pkg]++
		}
	}
	for k := edit.OpSet; k <= edit.OpSetCase; k++ {
		if (applied[k] == 0) != (k == edit.OpUnretire) {
			t.Errorf("operation %d applied %d times on the examples", k, applied[k])
		}
	}
	//canon:unordered each package is judged alone
	for pkg, n := range pkgs {
		if n == 0 {
			t.Errorf("%s: no operation applied, though Editable lets one edit it", pkg)
		}
	}
	t.Logf("applied by operation %v; by package %v", applied, pkgs)
}
