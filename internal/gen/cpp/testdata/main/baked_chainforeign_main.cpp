// The foreign-leaf chain (CODEGEN.md §5.3, §5.9, DECISIONS 432, 456): chain.a's byRow is keyed by a ref
// into chain.b's table, held as its id enum, and over by a ref into byRow. Each row is found at its
// address, holds the id it was built with, and an id the list lacks, or outside the enum, finds nothing.
#include <cstddef>
#include <cstdint>
#include <cstdio>

#include "chain/a/a.gen.h"
#include "chain/b/b.gen.h"

namespace a = chain::a;
namespace b = chain::b;

namespace {

int hits = 0;
int misses = 0;
int failures = 0;

void Expect(bool ok, const char* what) {
    if (!ok) {
        ++failures;
        std::printf("FAIL %s\n", what);
    }
}

template <class C, class KeyOf>
void FindsEach(const C& list, const char* what, KeyOf keyOf) {
    for (size_t i = 0; i < list.Len(); ++i) {
        ++hits;
        Expect(list.Find(keyOf(list.At(i))) == &list.At(i), what);
    }
}

template <class C>
void Lacks(const C& list, const char* what, b::RowId key) {
    ++misses;
    Expect(list.Find(key) == nullptr, what);
}

}  // namespace

int main() {
    FindsEach(a::GetByRow(), "byRow", [](const a::ByRowRow& r) { return r.GetRowKey(); });
    FindsEach(a::GetOver(), "over", [](const a::OverRow& r) { return r.GetOverKey(); });
    Expect(a::GetByRow().At(0).GetRowKey() == b::RowId::mid && a::GetByRow().At(2).GetRowKey() == b::RowId::beta, "byRow built");
    Expect(a::GetOver().At(0).GetOverKey() == b::RowId::zeta && a::GetOver().At(1).GetOverKey() == b::RowId::beta, "over built");
    for (int id : {4, 7, 255}) {  // outside chain.b's four ids
        Lacks(a::GetByRow(), "byRow", static_cast<b::RowId>(id));
        Lacks(a::GetOver(), "over", static_cast<b::RowId>(id));
    }
    Lacks(a::GetByRow(), "byRow", b::RowId::alpha);  // a row of chain.b that byRow lacks
    Lacks(a::GetOver(), "over", b::RowId::alpha);
    Lacks(a::GetOver(), "over", b::RowId::mid);  // a row of byRow that over lacks
    std::printf("byRow=%zu over=%zu\n", a::GetByRow().Len(), a::GetOver().Len());
    std::printf("hits=%d misses=%d failures=%d\n", hits, misses, failures);
    return failures == 0 ? 0 : 1;
}
