package lock

import (
	"cmp"
	"math/rand/v2"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
)

const (
	mergeSeedHi  = 13
	mergeSeedLo  = 7
	mergeRounds  = 400
	mergeMaxRun  = 60
	mergeModes   = 3
	comparePairs = 20000
	heldLineBase = 1000
	benchFacts   = 7000
	benchHolders = 2000
)

// Pools small enough that keys collide, retire and conflict; a.b.t is the start of a.b.t_x and
// f the start of f_1, so the comparator's prefix branch is drawn.
var (
	mergeNames   = []string{"a.b.t", "a.b.t_x", "a.b.u", "a.b.E", "a.b.F"}
	mergeHolders = []string{"k", "l", "m", "n", "retired"}
	mergeFields  = []string{"f", "f_1", "g"}
)

// referenceCompare is the comparator before mergeAll, cmp.Or over the parts, kept as the oracle.
func referenceCompare(a, b Fact) int {
	return cmp.Or(
		cmp.Compare(a.Kind.String(), b.Kind.String()),
		cmp.Compare(a.writtenName(), b.writtenName()),
		cmp.Or(cmp.Compare(rankOf(a.Value), rankOf(b.Value)), cmp.Compare(a.Value.Int, b.Value.Int), cmp.Compare(a.Value.Str, b.Value.Str)),
		cmp.Compare(a.Holder, b.Holder),
	)
}

func rankOf(v Value) int {
	if v.IsString {
		return 1
	}
	return 0
}

// referenceMerge is the one-by-one merge mergeAll replaced (LOCK.md §2.4), kept as the oracle.
func referenceMerge(f *File, fact Fact) bool {
	i, found := slices.BinarySearchFunc(f.facts, fact, referenceCompare)
	if !found {
		f.facts = slices.Insert(f.facts, i, fact)
		return true
	}
	if fact.Retired && !f.facts[i].Retired {
		f.facts[i].Retired = true
		return true
	}
	return false
}

// randomFact is a fact drawn from the pools, on line `line` (Span.Start), which shows which
// of two duplicates a merge kept.
func randomFact(rng *rand.Rand, line int) Fact {
	fact := Fact{
		Kind:   Kind(rng.IntN(len(kindNames))),
		Name:   mergeNames[rng.IntN(len(mergeNames))],
		Holder: mergeHolders[rng.IntN(len(mergeHolders))],
		Span:   source.Span{Start: source.Pos(line)},
	}
	switch fact.Kind {
	case KindEnum:
		fact.Value = Value{Int: int64(rng.IntN(len(mergeHolders)))}
		fact.Retired = rng.IntN(2) == 1
	case KindField:
		fact.Field = mergeFields[rng.IntN(len(mergeFields))]
		if rng.IntN(2) == 1 {
			fact.Value = Value{IsString: true, Str: mergeHolders[rng.IntN(len(mergeHolders))]}
		} else {
			fact.Value = Value{Int: int64(rng.IntN(len(mergeHolders)))}
		}
	default:
		fact.Retired = rng.IntN(2) == 1
	}
	return fact
}

// randomFacts is n random facts on lines 1..n.
func randomFacts(rng *rand.Rand, n int) []Fact {
	out := make([]Fact, n)
	for i := range out {
		out[i] = randomFact(rng, i+1)
	}
	return out
}

// heldBatch is facts already held, on new lines, with Retired flipped when flip and the kind can
// retire: a batch that changes nothing, or only retires, is what `changed` must be exact for.
func heldBatch(rng *rand.Rand, held []Fact, flip bool) []Fact {
	if len(held) == 0 {
		return nil
	}
	out := make([]Fact, 1+rng.IntN(mergeMaxRun))
	for i := range out {
		out[i] = held[rng.IntN(len(held))]
		out[i].Span = source.Span{Start: source.Pos(heldLineBase + i)}
		if flip && out[i].Kind != KindField {
			out[i].Retired = !out[i].Retired
		}
	}
	return out
}

// mergeBatch draws a round's batch: random facts, facts already held, or those with Retired flipped.
func mergeBatch(rng *rand.Rand, round int, held []Fact) []Fact {
	switch round % mergeModes {
	case 1:
		return heldBatch(rng, held, false)
	case 2:
		return heldBatch(rng, held, true)
	}
	return randomFacts(rng, rng.IntN(mergeMaxRun))
}

// LOCK.md §2.3, §2.4, IMPLEMENTATION-PLAN.md §7.6: mergeAll equals merging one by one, lines included.
func TestMergeAllIsSequentialMerge(t *testing.T) {
	rng := rand.New(rand.NewPCG(mergeSeedHi, mergeSeedLo))
	for round := range mergeRounds {
		held := randomFacts(rng, rng.IntN(mergeMaxRun))
		batch := mergeBatch(rng, round, held)
		want, got := New(propertyPackage), New(propertyPackage)
		for _, fact := range held {
			referenceMerge(want, fact)
		}
		got.mergeAll(slices.Clone(held))
		wantChanged := false
		for _, fact := range batch {
			wantChanged = referenceMerge(want, fact) || wantChanged
		}
		if gotChanged := got.mergeAll(slices.Clone(batch)); gotChanged != wantChanged {
			t.Fatalf("round %d: changed = %v, want %v", round, gotChanged, wantChanged)
		}
		if !slices.EqualFunc(got.facts, want.facts, func(a, b Fact) bool { return reflect.DeepEqual(a, b) }) {
			t.Fatalf("round %d: facts differ:\n%s\nwant\n%s", round, got.Format(), want.Format())
		}
	}
}

// LOCK.md §2.3: the comparator orders every pair as the cmp.Or comparator it replaced did.
func TestCompareFactsIsReferenceOrder(t *testing.T) {
	rng := rand.New(rand.NewPCG(mergeSeedLo, mergeSeedHi))
	for range comparePairs {
		a, b := randomFact(rng, 1), randomFact(rng, 1)
		if got, want := compareFacts(a, b), referenceCompare(a, b); cmp.Compare(got, 0) != cmp.Compare(want, 0) {
			t.Fatalf("compareFacts(%+v, %+v) = %d, want the sign of %d", a, b, got, want)
		}
	}
}

// LOCK.md §2.4: mergeAll of nothing changes nothing.
func TestMergeAllEmpty(t *testing.T) {
	f := New(propertyPackage)
	if f.mergeAll(nil) || len(f.facts) != 0 {
		t.Fatalf("empty batch changed %v", f.facts)
	}
}

// benchBatch is n distinct table facts in an order unlike the canonical one.
func benchBatch(n int) []Fact {
	rng := rand.New(rand.NewPCG(mergeSeedHi, mergeSeedLo))
	out := make([]Fact, n)
	for i := range out {
		out[i] = Fact{Kind: KindTable, Name: "a.b.t" + strconv.Itoa(rng.IntN(benchHolders)), Holder: "k" + strconv.Itoa(i)}
	}
	return out
}

// IMPLEMENTATION-PLAN.md §7.6: merging 7,000 facts, one by one (before) and in one pass (after).
func BenchmarkMerge(b *testing.B) {
	batch := benchBatch(benchFacts)
	b.Run("sequential", func(b *testing.B) {
		for range b.N {
			f := New(propertyPackage)
			for _, fact := range batch {
				referenceMerge(f, fact)
			}
		}
	})
	b.Run("mergeAll", func(b *testing.B) {
		for range b.N {
			New(propertyPackage).mergeAll(batch)
		}
	})
}
