// The enum-leaf chains (CODEGEN.md §5.2, §5.9, DECISIONS 432, 456): tones, ranks are keyed by an enum, picks
// and rpicks by a ref into them, over and rover by a ref into those. Tone's member names sort in an order
// that is not its positions; Rank's @codes (zulu 4, alpha 9, mid 1) are in neither. Each row is found at
// its address by its key, holds the key it was built with, and a key the list lacks finds nothing.
#include <cstddef>
#include <cstdio>

#include "chainenum.gen.h"

namespace c = demo::chainenum;

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

template <class C, class K>
void Lacks(const C& list, const char* what, K key) {
    ++misses;
    Expect(list.Find(key) == nullptr, what);
}

}  // namespace

int main() {
    FindsEach(c::GetPicks(), "picks", [](const c::PickRow& r) { return r.GetPickKey(); });
    FindsEach(c::GetOver(), "over", [](const c::OverRow& r) { return r.GetOverKey(); });
    FindsEach(c::GetTones(), "tones", [](const c::ToneRow& r) { return r.GetTone(); });
    Expect(c::GetPicks().At(0).GetPickKey() == c::Tone::mid && c::GetPicks().At(1).GetPickKey() == c::Tone::zed, "picks built");
    Expect(c::GetOver().At(0).GetOverKey() == c::Tone::zed && c::GetOver().At(1).GetOverKey() == c::Tone::mid, "over built");
    Lacks(c::GetPicks(), "picks", c::Tone::alpha);
    Lacks(c::GetOver(), "over", c::Tone::alpha);
    Lacks(c::GetPicks(), "picks", static_cast<c::Tone>(9));
    Lacks(c::GetOver(), "over", static_cast<c::Tone>(9));

    FindsEach(c::GetRpicks(), "rpicks", [](const c::RpickRow& r) { return r.GetRpickKey(); });
    FindsEach(c::GetRover(), "rover", [](const c::RoverRow& r) { return r.GetRoverKey(); });
    FindsEach(c::GetRanks(), "ranks", [](const c::RankRow& r) { return r.GetRank(); });
    Expect(c::GetRpicks().At(0).GetRpickKey() == c::Rank::zulu && c::GetRpicks().At(1).GetRpickKey() == c::Rank::mid &&
               c::GetRpicks().At(2).GetRpickKey() == c::Rank::alpha, "rpicks built");
    Expect(c::GetRover().At(0).GetRoverKey() == c::Rank::alpha && c::GetRover().At(1).GetRoverKey() == c::Rank::mid &&
               c::GetRover().At(2).GetRoverKey() == c::Rank::zulu, "rover built");
    Lacks(c::GetRpicks(), "rpicks", static_cast<c::Rank>(2));
    Lacks(c::GetRpicks(), "rpicks", static_cast<c::Rank>(200));
    Lacks(c::GetRover(), "rover", static_cast<c::Rank>(2));
    Lacks(c::GetRover(), "rover", static_cast<c::Rank>(200));

    std::printf("tones=%zu picks=%zu over=%zu ranks=%zu rpicks=%zu rover=%zu\n", c::GetTones().Len(), c::GetPicks().Len(), c::GetOver().Len(),
                c::GetRanks().Len(), c::GetRpicks().Len(), c::GetRover().Len());
    std::printf("hits=%d misses=%d failures=%d\n", hits, misses, failures);
    return failures == 0 ? 0 : 1;
}
