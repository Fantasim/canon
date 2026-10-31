// The bound constants of Probe are compile-time values (CODEGEN.md §5.15, DECISIONS 400): a hand-sized
// array and static_asserts name them.
#include "range.gen.h"

#include <chrono>
#include <cstdint>
#include <cstdio>

using demo::range::Probe;
using demo::range::ShapeCircle;

static_assert(Probe::kTagsLenMax == 3, "the list's count bound");
static_assert(Probe::kTagsElemMax == 5, "its element bound");
static_assert(Probe::kNMin == 1 && Probe::kNMax == 120, "both sides of n");
static_assert(Probe::kExEnd == 5, "an exclusive end");
static_assert(Probe::kF32Max == 0.1, "a Float32 bound is the binary64 value");
static_assert(Probe::kDMin == std::chrono::milliseconds{1000}, "a Duration bound in milliseconds");
static_assert(Probe::kMKeyLenMin == 1 && Probe::kMValueMax == 9, "a map's key and value");
static_assert(ShapeCircle::kRMax == 2, "a case field's bound");

int main() {
    int64_t tags[Probe::kTagsLenMax] = {1, 2, 3};
    std::printf("%s\n", tags[2] == 3 ? "ok" : "bad");
    return 0;
}
