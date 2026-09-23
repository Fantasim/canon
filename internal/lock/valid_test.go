package lock

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// noFiles is a diag.Files without any file: the property only reads whether Parse refused.
type noFiles struct{}

func (noFiles) Path(source.FileID) string                     { return "" }
func (noFiles) Position(source.FileID, source.Pos) (int, int) { return 0, 0 }
func (noFiles) Content(source.FileID) []byte                  { return nil }

const propertyPackage = "a.b"

// readsBack reports whether the line Format writes for fact parses into fact itself.
func readsBack(fact Fact) bool {
	data := []byte(header + lineBreak + fact.line() + lineBreak)
	f, ok := Parse(1, data, propertyPackage, diag.NewBag(noFiles{}, propertyPackage))
	if !ok || len(f.facts) != 1 {
		return false
	}
	got := f.facts[0]
	got.Span = source.Span{}
	return reflect.DeepEqual(got, fact)
}

// checkAddIsParseOfFormat: Add accepts a fact exactly when its Format line reads back as it.
func checkAddIsParseOfFormat(t *testing.T, fact Fact) {
	t.Helper()
	fact.Span = source.Span{}
	_, err := New(propertyPackage).Add(fact)
	if back := readsBack(fact); (err == nil) != back {
		t.Fatalf("Add(%+v) = %v, but the line %q reads back: %v", fact, err, fact.line(), back)
	}
}

// Pools of parts, each holding valid and invalid forms, so random facts hit every check.
var (
	poolNames   = []string{"a.b.t", "a.b.E", "a.b", "a.b.t.u", "x.b.t", "a.b.1t", "a.b._", "a.b.é", "a.b.retired", ""}
	poolFields  = []string{"", "f", "code", "1f", "f.g", "_", "f g"}
	poolHolders = []string{"k", "retired", "M_1", "", "_", "a b", `"k"`, "1k"}
	poolStrs    = []string{"", "x", "a b", "\"", "\\", "\x01", "\xff", "\u2028", "é"}
	poolInts    = []int64{0, 1, -1, 9, 1 << 62, -1 << 63}
)

// LOCK.md §2.2-§2.4 (MF-9): Add ⇔ Parse∘Format over random facts built from the pools.
func TestAddIsParseOfFormat(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	pick := func(n int) int { return rng.IntN(n) }
	for range 20000 {
		fact := Fact{
			Kind:    Kind(pick(len(kindNames) + 1)),
			Name:    poolNames[pick(len(poolNames))],
			Field:   poolFields[pick(len(poolFields))],
			Holder:  poolHolders[pick(len(poolHolders))],
			Retired: pick(2) == 1,
			Value:   Value{IsString: pick(2) == 1, Int: poolInts[pick(len(poolInts))], Str: poolStrs[pick(len(poolStrs))]},
		}
		checkAddIsParseOfFormat(t, fact)
	}
}

// LOCK.md §2.3: a lock of facts Add accepted formats, parses and formats to the same bytes.
func TestAcceptedLockReadsBack(t *testing.T) {
	f := New(propertyPackage)
	for _, fact := range []Fact{
		{Kind: KindTable, Name: "a.b.t", Holder: "retired", Retired: true},
		{Kind: KindEnum, Name: "a.b.E", Value: Value{Int: -1 << 63}, Holder: "M"},
		{Kind: KindField, Name: "a.b.t", Field: "f", Value: Value{IsString: true, Str: "a  b\u2028\x01"}, Holder: "k"},
	} {
		if _, err := f.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	again, ok := Parse(1, f.Format(), propertyPackage, diag.NewBag(noFiles{}, propertyPackage))
	if !ok || string(again.Format()) != string(f.Format()) {
		t.Fatalf("read back %v:\n%s\nfrom\n%s", ok, again.Format(), f.Format())
	}
}

// IMPLEMENTATION-PLAN.md §7.7: the same property, fuzzed.
func FuzzAdd(f *testing.F) {
	f.Add(uint8(KindTable), "a.b.t", "", false, int64(0), "", "k", true)
	f.Add(uint8(KindEnum), "a.b.E", "", true, int64(0), "x", "M", false)
	f.Add(uint8(KindField), "a.b.t", "f", true, int64(0), "\xff", "k", false)
	f.Add(uint8(KindField), "a.b.t", "f", false, int64(-3), "", "k", true)
	f.Fuzz(func(t *testing.T, kind uint8, name, field string, isString bool, n int64, str, holder string, retired bool) {
		checkAddIsParseOfFormat(t, Fact{
			Kind: Kind(kind), Name: name, Field: field, Holder: holder, Retired: retired,
			Value: Value{IsString: isString, Int: n, Str: str},
		})
	})
}
