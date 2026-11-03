// The nested-ref fixture (CODEGEN.md §5.9, DECISIONS 432, 456): a keyed list keyed by a ref into a keyed
// list keyed by a ref into a table finds each of its rows by the table's id, and nothing for an id it lacks.
#include <cstddef>
#include <cstdio>

#include "nestref.gen.h"

namespace n = demo::nestref;

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
void FindsEach(const C& c, const char* what, KeyOf keyOf) {
    for (size_t i = 0; i < c.Len(); ++i) {
        ++hits;
        Expect(c.Find(keyOf(c.At(i))) == &c.At(i), what);
    }
}

template <class C>
void Lacks(const C& c, const char* what, n::KindId key) {
    ++misses;
    Expect(c.Find(key) == nullptr, what);
}

}  // namespace

int main() {
    FindsEach(n::GetByKind(), "byKind", [](const n::KindRow& r) { return r.GetKindKey(); });
    FindsEach(n::GetOuter(), "outer", [](const n::OuterRow& r) { return r.GetOuterKey(); });
    FindsEach(n::GetDeep(), "deep", [](const n::DeepRow& r) { return r.GetDeepKey(); });
    Expect(n::GetByKind().At(0).GetKindKey() == n::KindId::mid && n::GetByKind().At(2).GetKindKey() == n::KindId::beta, "byKind built");
    Expect(n::GetOuter().At(0).GetOuterKey() == n::KindId::beta && n::GetOuter().At(2).GetOuterKey() == n::KindId::zeta, "outer built");
    Expect(n::GetDeep().At(0).GetDeepKey() == n::KindId::zeta && n::GetDeep().At(2).GetDeepKey() == n::KindId::mid, "deep built");
    Lacks(n::GetOuter(), "outer", n::KindId::alpha);
    Lacks(n::GetOuter(), "outer", static_cast<n::KindId>(9));
    Lacks(n::GetDeep(), "deep", n::KindId::alpha);
    Lacks(n::GetDeep(), "deep", static_cast<n::KindId>(9));
    Lacks(n::GetByKind(), "byKind", n::KindId::alpha);
    Lacks(n::GetByKind(), "byKind", static_cast<n::KindId>(9));
    std::printf("byKind=%zu outer=%zu deep=%zu\n", n::GetByKind().Len(), n::GetOuter().Len(), n::GetDeep().Len());
    std::printf("hits=%d misses=%d failures=%d\n", hits, misses, failures);
    return failures == 0 ? 0 : 1;
}
