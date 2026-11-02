// The constants of a variant's kind are the kind enum's enumerators (CODEGEN.md §4.4, §5.1, DECISIONS 438):
// compile-time values a static_assert can name; the lists read at run time.
#include "demo/kinds/out/kinds.gen.h"

#include <cstdio>

using demo::kinds::BK;
using demo::kinds::K;
using demo::kinds::ShapeKind;

static_assert(K == ShapeKind::dot, "a kind constant is its enumerator");
static_assert(BK == demo::reward::RewardKind::gold, "another package's kind is qualified");

int main() {
    const bool ok = demo::kinds::KS.size() == 2 && demo::kinds::KS[1] == ShapeKind::box &&
                    demo::kinds::Seven.size() == 7 && demo::kinds::Seven[6] == ShapeKind::dot &&
                    demo::kinds::Eight.size() == 8 && demo::kinds::Eight[7] == ShapeKind::box;
    std::printf("%s\n", ok ? "ok" : "bad");
    return ok ? 0 : 1;
}
