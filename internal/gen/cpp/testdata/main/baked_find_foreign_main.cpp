// A keyed list of find.a keyed by `ref b.rows`, held as find.b's id enum (DECISIONS 432): Find
// finds each row at its address, and nothing for an id find.b's table lacks or outside its enum.
#include <cstdint>
#include <cstdio>

#include "find/a/a.gen.h"
#include "find/b/b.gen.h"

namespace a = find::a;
namespace b = find::b;

int main() {
    int hits = 0;
    int misses = 0;
    int failures = 0;
    const a::Things& things = a::GetThings();
    for (size_t i = 0; i < things.Len(); ++i) {
        ++hits;
        if (things.Find(things.At(i).GetRowKey()) != &things.At(i)) {
            ++failures;
            std::printf("FAIL hit %zu\n", i);
        }
    }
    for (int id : {4, 7, 255}) {  // outside find.b's four ids
        ++misses;
        if (things.Find(static_cast<b::RowId>(id)) != nullptr) {
            ++failures;
            std::printf("FAIL miss %d\n", id);
        }
    }
    ++misses;  // alpha is a row of find.b that things lacks
    if (things.Find(b::RowId::alpha) != nullptr) {
        ++failures;
        std::printf("FAIL miss alpha\n");
    }
    std::printf("things=%zu hits=%d misses=%d failures=%d\n", things.Len(), hits, misses, failures);
    return failures == 0 ? 0 : 1;
}
